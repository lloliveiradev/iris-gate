package idempotency_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	identityapp "github.com/iris-gate/iris-gate/libs/identity/application"
	identitydomain "github.com/iris-gate/iris-gate/libs/identity/domain"
	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
	"github.com/iris-gate/iris-gate/libs/payments/infra/idempotency"
)

// Fake values shared by the middleware tests.
const (
	testScope   = "create_payment_intent"
	testKey     = "idem-key-test-0001"
	requestBody = `{"amount_cents":1000}`
	otherBody   = `{"amount_cents":2000}`
	handlerBody = `{"id":"pi_test"}`

	// fakeAPIKeyPrefix and fakeAPIKeySecret form the test bearer token;
	// acceptAnySecret accepts any secret.
	fakeAPIKeyPrefix = "prefixidem"
	fakeAPIKeySecret = "secret"
)

// Wire values the middleware answers with.
const (
	replayHeader         = "Idempotent-Replay"
	conflictTitle        = "idempotency_conflict"
	inProgressTitle      = "idempotency_in_progress"
	invalidKeyTitle      = "invalid_idempotency_key"
	missingKeyTitle      = "missing_idempotency_key"
	invalidBodyTitle     = "invalid_body"
	requestTooLargeTitle = "request_too_large"
	internalTitle        = "internal"
)

// Limits the middleware enforces. They are written out here, not imported, so
// a change to one of them fails these tests.
const (
	minKeyLength         = 8
	maxKeyLength         = 128
	maxRequestBodyBytes  = 64 << 10
	maxResponseBodyBytes = 1 << 20

	// responseChunkBytes is the size of each write of tooBigHandler.
	responseChunkBytes = 32 << 10
)

// racingRequests is how many first requests with one key race in
// TestMiddleware_ConcurrentFirstRequests_RunHandlerOnce.
const racingRequests = 20

// waitTimeout bounds every wait on another goroutine, so a broken middleware
// fails the test instead of hanging it.
const waitTimeout = 5 * time.Second

var (
	errStoreDown     = errors.New("store unavailable")
	errHandlerPanic  = errors.New("handler panicked")
	errNoReservation = errors.New("no reservation for key")
	// errBodyRead carries text a client must never see in an answer.
	errBodyRead = errors.New("connection reset by peer, internal-detail-xyz")
)

// ---------- identity stubs: the real RequireAPIKey, one in-memory principal ----------

type oneAPIKey struct {
	identityapp.APIKeyRepo
	key identitydomain.APIKey
}

func (s oneAPIKey) GetByPrefix(_ context.Context, prefix string) (identitydomain.APIKey, error) {
	if prefix != s.key.Prefix {
		return identitydomain.APIKey{}, identitydomain.ErrAPIKeyNotFound
	}
	return s.key, nil
}

func (oneAPIKey) TouchLastUsed(context.Context, uuid.UUID, time.Time) error { return nil }

type oneMerchant struct {
	identityapp.MerchantRepo
	merchant identitydomain.Merchant
}

func (s oneMerchant) GetByID(_ context.Context, id uuid.UUID) (identitydomain.Merchant, error) {
	if id != s.merchant.ID {
		return identitydomain.Merchant{}, identitydomain.ErrMerchantNotFound
	}
	return s.merchant, nil
}

type acceptAnySecret struct{ identityapp.Hasher }

func (acceptAnySecret) Verify(string, string) (bool, error) { return true, nil }

// authenticateAs returns RequireAPIKey wired to stubs that know one active
// merchant, and the bearer token that authenticates as it.
func authenticateAs(merchantID uuid.UUID) (auth func(http.Handler) http.Handler, token string) {
	key := identitydomain.APIKey{
		ID: uuid.New(), MerchantID: merchantID, Prefix: fakeAPIKeyPrefix, Status: identitydomain.APIKeyActive,
	}
	merchant := identitydomain.Merchant{ID: merchantID, Status: identitydomain.MerchantActive}
	uc := identityapp.NewAuthenticateAPIKeyUseCase(
		oneAPIKey{key: key}, oneMerchant{merchant: merchant}, acceptAnySecret{}, identityapp.SystemClock{})
	return identityhttp.RequireAPIKey(uc), identitydomain.FormatToken(fakeAPIKeyPrefix, fakeAPIKeySecret)
}

// ---------- in-memory store ----------

// memStore is an in-memory Store. Reserve is atomic under mu, like the
// Postgres INSERT ... ON CONFLICT. Every call fails on a done context, as a
// database driver would.
type memStore struct {
	mu           sync.Mutex
	records      map[string]idempotency.Record
	reserveCalls int
	reserveErr   error
	completeErr  error
}

func newMemStore() *memStore { return &memStore{records: map[string]idempotency.Record{}} }

func recordKey(merchantID uuid.UUID, scope, key string) string {
	return merchantID.String() + "|" + scope + "|" + key
}

func (s *memStore) Reserve(ctx context.Context, merchantID uuid.UUID, scope, key string, requestHash []byte) (*idempotency.Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reserveCalls++
	if s.reserveErr != nil {
		return nil, s.reserveErr
	}
	k := recordKey(merchantID, scope, key)
	if rec, ok := s.records[k]; ok {
		return &rec, nil
	}
	s.records[k] = idempotency.Record{RequestHash: bytes.Clone(requestHash), InProgress: true}
	return nil, nil
}

func (s *memStore) Complete(ctx context.Context, merchantID uuid.UUID, scope, key string, status int, body []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.completeErr != nil {
		return s.completeErr
	}
	k := recordKey(merchantID, scope, key)
	rec, ok := s.records[k]
	if !ok || !rec.InProgress {
		return errNoReservation
	}
	rec.InProgress, rec.ResponseStatus, rec.ResponseBody = false, status, bytes.Clone(body)
	s.records[k] = rec
	return nil
}

func (s *memStore) Release(ctx context.Context, merchantID uuid.UUID, scope, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := recordKey(merchantID, scope, key)
	if rec, ok := s.records[k]; ok && rec.InProgress {
		delete(s.records, k)
	}
	return nil
}

// reservations is how many times Reserve was called, whatever its outcome.
func (s *memStore) reservations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reserveCalls
}

// record returns what the store holds for testKey.
func (s *memStore) record(merchantID uuid.UUID) (idempotency.Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[recordKey(merchantID, testScope, testKey)]
	return rec, ok
}

// ---------- handlers ----------

// countingHandler answers status with handlerBody and counts its calls.
type countingHandler struct {
	status int
	calls  atomic.Int32
}

func (h *countingHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.calls.Add(1)
	writeResponse(w, h.status)
}

// blockingHandler signals entered when a call starts, then answers 201 once
// release is closed.
type blockingHandler struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func newBlockingHandler() *blockingHandler {
	return &blockingHandler{entered: make(chan struct{}, racingRequests), release: make(chan struct{})}
}

func (h *blockingHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.calls.Add(1)
	h.entered <- struct{}{}
	<-h.release
	writeResponse(w, http.StatusCreated)
}

func writeResponse(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, handlerBody)
}

// sizedHandler answers 201 with size bytes, written in responseChunkBytes
// chunks, and counts its calls.
type sizedHandler struct {
	size  int
	calls atomic.Int32
}

func (h *sizedHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.calls.Add(1)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	chunk := bytes.Repeat([]byte("a"), responseChunkBytes)
	for written := 0; written < h.size; written += responseChunkBytes {
		if _, err := w.Write(chunk[:min(responseChunkBytes, h.size-written)]); err != nil {
			return
		}
	}
}

// spyBody is an empty request body that records whether it was read.
type spyBody struct{ read atomic.Bool }

func (b *spyBody) Read([]byte) (int, error) {
	b.read.Store(true)
	return 0, io.EOF
}

// failingBody is a request body whose read fails with err.
type failingBody struct{ err error }

func (b failingBody) Read([]byte) (int, error) { return 0, b.err }

// ---------- harness ----------

type harness struct {
	store    *memStore
	merchant uuid.UUID
	token    string
	handler  http.Handler
}

// newHarness mounts the real RequireAPIKey and the middleware in front of next.
func newHarness(next http.Handler) *harness {
	h := &harness{store: newMemStore(), merchant: uuid.New()}
	auth, token := authenticateAs(h.merchant)
	h.token = token
	h.handler = auth(idempotency.Middleware(h.store, testScope)(next))
	return h
}

func (h *harness) post(ctx context.Context, body string) *httptest.ResponseRecorder {
	return h.postWithKey(ctx, testKey, strings.NewReader(body))
}

// postWithKey sends body under key; an empty key sends no Idempotency-Key.
func (h *harness) postWithKey(ctx context.Context, key string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/payment-intents", body)
	req.Header.Set("Authorization", "Bearer "+h.token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

// postCatchingPanic sends requestBody and returns what the chain panicked with.
func (h *harness) postCatchingPanic() (panicked any) {
	defer func() { panicked = recover() }()
	h.post(context.Background(), requestBody)
	return nil
}

// startInFlight sends requestBody to a blockingHandler and returns once the
// handler runs. finish lets the handler answer and returns its response.
func startInFlight(t *testing.T, h *harness, next *blockingHandler) (finish func() *httptest.ResponseRecorder) {
	t.Helper()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- h.post(context.Background(), requestBody) }()
	receive(t, next.entered)
	return func() *httptest.ResponseRecorder {
		close(next.release)
		return receive(t, done)
	}
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(waitTimeout):
		t.Fatal("timed out waiting for another goroutine")
		var zero T
		return zero
	}
}

func problemTitle(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var problem struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem %q: %v", rec.Body.String(), err)
	}
	return problem.Title
}

func requireProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, title string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body.String())
	}
	if got := problemTitle(t, rec); got != title {
		t.Fatalf("title = %q, want %q", got, title)
	}
}

func requireOriginal(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusCreated || rec.Body.String() != handlerBody || rec.Header().Get(replayHeader) != "" {
		t.Fatalf("response = %d %q replay=%q, want the handler's 201 %q", rec.Code, rec.Body.String(), rec.Header().Get(replayHeader), handlerBody)
	}
}

func requireReplay(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusCreated || rec.Body.String() != handlerBody || rec.Header().Get(replayHeader) != "true" {
		t.Fatalf("response = %d %q replay=%q, want a replay of 201 %q", rec.Code, rec.Body.String(), rec.Header().Get(replayHeader), handlerBody)
	}
}

// ---------- tests ----------

type unreachableStore struct{}

func (unreachableStore) Reserve(context.Context, uuid.UUID, string, string, []byte) (*idempotency.Record, error) {
	panic("store must not be reserved without a principal")
}

func (unreachableStore) Complete(context.Context, uuid.UUID, string, string, int, []byte) error {
	panic("store must not be completed without a principal")
}

func (unreachableStore) Release(context.Context, uuid.UUID, string, string) error {
	panic("store must not be released without a principal")
}

func TestMiddleware_WithoutPrincipal_Returns401AndSkipsHandler(t *testing.T) {
	t.Parallel()
	var calls int32
	mw := idempotency.Middleware(unreachableStore{}, "test")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", testKey)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if calls != 0 {
		t.Fatalf("handler called %d times, want 0", calls)
	}
}

func TestMiddleware_FirstRequest_RunsHandlerAndStoresResponse(t *testing.T) {
	t.Parallel()
	next := &countingHandler{status: http.StatusCreated}
	h := newHarness(next)

	requireOriginal(t, h.post(context.Background(), requestBody))

	stored, ok := h.store.record(h.merchant)
	wantHash := sha256.Sum256([]byte(requestBody))
	if !ok || stored.InProgress || stored.ResponseStatus != http.StatusCreated ||
		string(stored.ResponseBody) != handlerBody || !bytes.Equal(stored.RequestHash, wantHash[:]) {
		t.Fatalf("stored record = %+v (found %v), want the completed 201 with the request hash", stored, ok)
	}
	if n := next.calls.Load(); n != 1 {
		t.Fatalf("handler calls = %d, want 1", n)
	}
}

func TestMiddleware_SameKeySameBody_ReplaysStoredResponse(t *testing.T) {
	t.Parallel()
	next := &countingHandler{status: http.StatusCreated}
	h := newHarness(next)

	requireOriginal(t, h.post(context.Background(), requestBody))
	requireReplay(t, h.post(context.Background(), requestBody))
	if n := next.calls.Load(); n != 1 {
		t.Fatalf("handler calls = %d, want 1", n)
	}
}

func TestMiddleware_SameKeyDifferentBody_Returns409Conflict(t *testing.T) {
	t.Parallel()
	next := &countingHandler{status: http.StatusCreated}
	h := newHarness(next)

	requireOriginal(t, h.post(context.Background(), requestBody))
	requireProblem(t, h.post(context.Background(), otherBody), http.StatusConflict, conflictTitle)
	if n := next.calls.Load(); n != 1 {
		t.Fatalf("handler calls = %d, want 1", n)
	}
}

func TestMiddleware_RequestInFlight_Returns409InProgress(t *testing.T) {
	t.Parallel()
	next := newBlockingHandler()
	h := newHarness(next)

	finish := startInFlight(t, h, next)
	requireProblem(t, h.post(context.Background(), requestBody), http.StatusConflict, inProgressTitle)
	requireProblem(t, h.post(context.Background(), otherBody), http.StatusConflict, conflictTitle)

	requireOriginal(t, finish())
	requireReplay(t, h.post(context.Background(), requestBody))
	if n := next.calls.Load(); n != 1 {
		t.Fatalf("handler calls = %d, want 1", n)
	}
}

func TestMiddleware_HandlerFailure_ReleasesKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int // the first call's status; 0 means it panics
	}{
		{name: "server error", status: http.StatusInternalServerError},
		{name: "client error", status: http.StatusBadRequest},
		{name: "panic"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) > 1 {
					writeResponse(w, http.StatusCreated)
					return
				}
				if tc.status == 0 {
					panic(errHandlerPanic)
				}
				writeResponse(w, tc.status)
			})
			h := newHarness(next)

			if tc.status == 0 {
				if p := h.postCatchingPanic(); p != errHandlerPanic {
					t.Fatalf("panic = %v, want %v", p, errHandlerPanic)
				}
			} else if first := h.post(context.Background(), requestBody); first.Code != tc.status {
				t.Fatalf("first status = %d, want %d", first.Code, tc.status)
			}
			if rec, ok := h.store.record(h.merchant); ok {
				t.Fatalf("key still held after the failure: %+v", rec)
			}

			requireOriginal(t, h.post(context.Background(), requestBody))
			if n := calls.Load(); n != 2 {
				t.Fatalf("handler calls = %d, want 2 (the retry runs it again)", n)
			}
		})
	}
}

func TestMiddleware_ClientGone_StillSettlesKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		status    int
		wantFound bool
	}{
		{name: "handler succeeded: response stored", status: http.StatusCreated, wantFound: true},
		{name: "handler failed: key released", status: http.StatusInternalServerError, wantFound: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				cancel() // the client disconnects while the handler runs
				writeResponse(w, tc.status)
			})
			h := newHarness(next)

			h.post(ctx, requestBody)

			rec, found := h.store.record(h.merchant)
			if found != tc.wantFound || rec.InProgress {
				t.Fatalf("record = %+v (found %v), want found=%v and never left in progress", rec, found, tc.wantFound)
			}
		})
	}
}

func TestMiddleware_CompleteFails_KeepsKeyReserved(t *testing.T) {
	t.Parallel()
	next := &countingHandler{status: http.StatusCreated}
	h := newHarness(next)
	h.store.completeErr = errStoreDown

	requireOriginal(t, h.post(context.Background(), requestBody))
	requireProblem(t, h.post(context.Background(), requestBody), http.StatusConflict, inProgressTitle)
	if n := next.calls.Load(); n != 1 {
		t.Fatalf("handler calls = %d, want 1: a retry must not repeat the side effect", n)
	}
}

func TestMiddleware_ReserveFails_Returns500AndSkipsHandler(t *testing.T) {
	t.Parallel()
	next := &countingHandler{status: http.StatusCreated}
	h := newHarness(next)
	h.store.reserveErr = errStoreDown

	if rec := h.post(context.Background(), requestBody); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if n := next.calls.Load(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func TestMiddleware_ConcurrentFirstRequests_RunHandlerOnce(t *testing.T) {
	t.Parallel()
	next := newBlockingHandler()
	h := newHarness(next)

	results := make(chan *httptest.ResponseRecorder, racingRequests)
	start := make(chan struct{})
	for range racingRequests {
		go func() {
			<-start
			results <- h.post(context.Background(), requestBody)
		}()
	}
	close(start)

	// One request holds the key and blocks in the handler; every other one
	// answers without waiting for it.
	receive(t, next.entered)
	for range racingRequests - 1 {
		requireProblem(t, receive(t, results), http.StatusConflict, inProgressTitle)
	}
	close(next.release)
	requireOriginal(t, receive(t, results))

	if n := next.calls.Load(); n != 1 {
		t.Fatalf("handler calls = %d, want 1", n)
	}
	requireReplay(t, h.post(context.Background(), requestBody))
}

// requireNoReservation fails if a rejected request touched the store or ran
// the handler.
func requireNoReservation(t *testing.T, h *harness, next *countingHandler) {
	t.Helper()
	if n := h.store.reservations(); n != 0 {
		t.Fatalf("reservations = %d, want 0: the request was rejected before reserving", n)
	}
	if n := next.calls.Load(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func TestMiddleware_IdempotencyKeyLength_EnforcesContractBounds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       string
		wantTitle string // empty: the request is accepted
	}{
		{name: "missing", key: "", wantTitle: missingKeyTitle},
		{name: "one under the minimum", key: strings.Repeat("k", minKeyLength-1), wantTitle: invalidKeyTitle},
		{name: "minimum", key: strings.Repeat("k", minKeyLength)},
		{name: "maximum", key: strings.Repeat("k", maxKeyLength)},
		{name: "one over the maximum", key: strings.Repeat("k", maxKeyLength+1), wantTitle: invalidKeyTitle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			next := &countingHandler{status: http.StatusCreated}
			h := newHarness(next)
			body := &spyBody{}

			rec := h.postWithKey(context.Background(), tc.key, body)

			if tc.wantTitle == "" {
				requireOriginal(t, rec)
				if n := h.store.reservations(); n != 1 {
					t.Fatalf("reservations = %d, want 1", n)
				}
				return
			}
			requireProblem(t, rec, http.StatusBadRequest, tc.wantTitle)
			if body.read.Load() {
				t.Fatal("body was read before the key was validated")
			}
			requireNoReservation(t, h, next)
		})
	}
}

func TestMiddleware_RequestBodyAtLimit_IsAcceptedAndReachesHandler(t *testing.T) {
	t.Parallel()
	var received int
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("handler read body: %v", err)
		}
		received = len(body)
		writeResponse(w, http.StatusCreated)
	})
	h := newHarness(next)

	requireOriginal(t, h.post(context.Background(), strings.Repeat("a", maxRequestBodyBytes)))

	if received != maxRequestBodyBytes {
		t.Fatalf("handler received %d bytes, want %d", received, maxRequestBodyBytes)
	}
	if n := h.store.reservations(); n != 1 {
		t.Fatalf("reservations = %d, want 1", n)
	}
}

func TestMiddleware_RequestBodyOverLimit_Returns413WithoutReserving(t *testing.T) {
	t.Parallel()
	next := &countingHandler{status: http.StatusCreated}
	h := newHarness(next)

	rec := h.post(context.Background(), strings.Repeat("a", maxRequestBodyBytes+1))

	requireProblem(t, rec, http.StatusRequestEntityTooLarge, requestTooLargeTitle)
	if strings.Contains(rec.Body.String(), "http:") {
		t.Fatalf("answer echoes the reader error: %s", rec.Body.String())
	}
	requireNoReservation(t, h, next)

	// The key was never reserved: the same key with a valid body runs.
	requireOriginal(t, h.post(context.Background(), requestBody))
	if n := next.calls.Load(); n != 1 {
		t.Fatalf("handler calls = %d, want 1", n)
	}
}

func TestMiddleware_UnreadableBody_Returns400WithFixedDetailAndWithoutReserving(t *testing.T) {
	t.Parallel()
	next := &countingHandler{status: http.StatusCreated}
	h := newHarness(next)

	rec := h.postWithKey(context.Background(), testKey, failingBody{err: errBodyRead})

	requireProblem(t, rec, http.StatusBadRequest, invalidBodyTitle)
	if strings.Contains(rec.Body.String(), errBodyRead.Error()) {
		t.Fatalf("answer echoes the reader error: %s", rec.Body.String())
	}
	requireNoReservation(t, h, next)
}

func TestMiddleware_ResponseAtLimit_IsStoredAndReplayed(t *testing.T) {
	t.Parallel()
	next := &sizedHandler{size: maxResponseBodyBytes}
	h := newHarness(next)

	first := h.post(context.Background(), requestBody)
	if first.Code != http.StatusCreated || first.Body.Len() != maxResponseBodyBytes {
		t.Fatalf("response = %d with %d bytes, want 201 with %d", first.Code, first.Body.Len(), maxResponseBodyBytes)
	}
	stored, ok := h.store.record(h.merchant)
	if !ok || stored.InProgress || len(stored.ResponseBody) != maxResponseBodyBytes {
		t.Fatalf("stored record = in progress %v, %d bytes (found %v), want the completed response", stored.InProgress, len(stored.ResponseBody), ok)
	}

	replay := h.post(context.Background(), requestBody)
	if replay.Code != http.StatusCreated || replay.Header().Get(replayHeader) != "true" || replay.Body.Len() != maxResponseBodyBytes {
		t.Fatalf("replay = %d replay=%q with %d bytes", replay.Code, replay.Header().Get(replayHeader), replay.Body.Len())
	}
	if n := next.calls.Load(); n != 1 {
		t.Fatalf("handler calls = %d, want 1", n)
	}
}

func TestMiddleware_ResponseOverLimit_KeepsKeyReservedAndReturns500(t *testing.T) {
	t.Parallel()
	next := &sizedHandler{size: maxResponseBodyBytes + 1}
	h := newHarness(next)

	requireProblem(t, h.post(context.Background(), requestBody), http.StatusInternalServerError, internalTitle)
	stored, ok := h.store.record(h.merchant)
	if !ok || !stored.InProgress || len(stored.ResponseBody) != 0 {
		t.Fatalf("stored record = in progress %v, %d bytes (found %v), want the key still reserved and no response stored", stored.InProgress, len(stored.ResponseBody), ok)
	}

	// The handler already ran, so a retry must not run it again.
	requireProblem(t, h.post(context.Background(), requestBody), http.StatusConflict, inProgressTitle)
	if n := next.calls.Load(); n != 1 {
		t.Fatalf("handler calls = %d, want 1", n)
	}
}
