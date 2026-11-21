package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	identityapp "github.com/iris-gate/iris-gate/libs/identity/application"
	identitydomain "github.com/iris-gate/iris-gate/libs/identity/domain"
	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/domain"
	"github.com/iris-gate/iris-gate/libs/payments/infra/httpapi"
	"github.com/iris-gate/iris-gate/libs/payments/infra/idempotency"
	"github.com/iris-gate/iris-gate/libs/payments/testfixtures"
)

const (
	notFoundTitle = "payment_intent_not_found"
	// fakeAPIKeySecret is the secret part of the test API keys; the hasher
	// stub accepts any secret.
	fakeAPIKeySecret = "secret"

	// Idempotency keys must be 8 to 128 characters (the middleware enforces
	// the contract bounds). sharedIdempotencyKey is the key two merchants use.
	testIdempotencyKey   = "idem-key-test-0001"
	sharedIdempotencyKey = "idem-key-shared-0001"
)

var (
	createBody = `{"amount":{"amount_cents":1000,"currency":"BRL"},"card_token":"` + testfixtures.TokenApproved + `"}`
	otherBody  = `{"amount":{"amount_cents":2000,"currency":"BRL"},"card_token":"` + testfixtures.TokenApproved + `"}`
)

// ---------- identity stubs: real RequireAPIKey, in-memory principals ----------

type stubKeys struct {
	identityapp.APIKeyRepo
	byPrefix map[string]identitydomain.APIKey
}

func (s stubKeys) GetByPrefix(_ context.Context, prefix string) (identitydomain.APIKey, error) {
	k, ok := s.byPrefix[prefix]
	if !ok {
		return identitydomain.APIKey{}, identitydomain.ErrAPIKeyNotFound
	}
	return k, nil
}
func (stubKeys) TouchLastUsed(context.Context, uuid.UUID, time.Time) error { return nil }

type stubMerchants struct {
	identityapp.MerchantRepo
	byID map[uuid.UUID]identitydomain.Merchant
}

func (s stubMerchants) GetByID(_ context.Context, id uuid.UUID) (identitydomain.Merchant, error) {
	m, ok := s.byID[id]
	if !ok {
		return identitydomain.Merchant{}, identitydomain.ErrMerchantNotFound
	}
	return m, nil
}

type acceptAllHasher struct{ identityapp.Hasher }

func (acceptAllHasher) Verify(string, string) (bool, error) { return true, nil }

type stubClock struct{}

func (stubClock) Now() time.Time { return time.Now().UTC() }

// ---------- payments in-memory fakes ----------

type memRepo struct {
	mu      sync.Mutex
	intents map[uuid.UUID]domain.PaymentIntent
	outbox  int
	// updateErr, when set, fails every update without writing.
	updateErr error
}

func (r *memRepo) CreateWithOutbox(_ context.Context, p domain.PaymentIntent, _ application.OutboxMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.intents[p.ID] = p
	r.outbox++
	return nil
}

func (r *memRepo) UpdateWithOutbox(_ context.Context, p domain.PaymentIntent, _ application.OutboxMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
	r.intents[p.ID] = p
	r.outbox++
	return nil
}

func (r *memRepo) GetByID(_ context.Context, merchantID, id uuid.UUID) (domain.PaymentIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.intents[id]
	if !ok || p.MerchantID != merchantID {
		return domain.PaymentIntent{}, domain.ErrPaymentIntentNotFound
	}
	return p, nil
}

func (r *memRepo) ListByMerchant(_ context.Context, merchantID uuid.UUID, _ int) ([]domain.PaymentIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.PaymentIntent
	for _, p := range r.intents {
		if p.MerchantID == merchantID {
			out = append(out, p)
		}
	}
	return out, nil
}

type memIdempotencyStore struct {
	mu      sync.Mutex
	records map[string]*idempotency.Record
}

func idemKey(m uuid.UUID, scope, key string) string { return m.String() + "|" + scope + "|" + key }

func (s *memIdempotencyStore) Reserve(_ context.Context, m uuid.UUID, scope, key string, hash []byte) (*idempotency.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := idemKey(m, scope, key)
	if rec, ok := s.records[k]; ok {
		stored := *rec
		return &stored, nil
	}
	s.records[k] = &idempotency.Record{RequestHash: hash, InProgress: true}
	return nil, nil
}

func (s *memIdempotencyStore) Complete(_ context.Context, m uuid.UUID, scope, key string, status int, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.records[idemKey(m, scope, key)]
	rec.InProgress, rec.ResponseStatus, rec.ResponseBody = false, status, body
	return nil
}

func (s *memIdempotencyStore) Release(_ context.Context, m uuid.UUID, scope, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := idemKey(m, scope, key)
	if rec, ok := s.records[k]; ok && rec.InProgress {
		delete(s.records, k)
	}
	return nil
}

type countingAcquirer struct {
	mu      sync.Mutex
	calls   int
	decline bool
}

func (a *countingAcquirer) Authorize(_ context.Context, in application.AuthorizeInput) (application.AuthorizeOutput, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	return application.AuthorizeOutput{AcquirerRef: "ref-" + in.IntentID.String(), Approved: !a.decline}, nil
}

type seqIDs struct{}

func (seqIDs) NewUUID() uuid.UUID { return uuid.New() }

// ---------- harness ----------

type harness struct {
	router   http.Handler
	repo     *memRepo
	acquirer *countingAcquirer
	tokenA   string
	tokenB   string
	merchA   uuid.UUID
	merchB   uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		repo:     &memRepo{intents: map[uuid.UUID]domain.PaymentIntent{}},
		acquirer: &countingAcquirer{},
		merchA:   uuid.New(),
		merchB:   uuid.New(),
	}
	keys := stubKeys{byPrefix: map[string]identitydomain.APIKey{}}
	merchants := stubMerchants{byID: map[uuid.UUID]identitydomain.Merchant{}}
	issue := func(id uuid.UUID, prefix string) string {
		merchants.byID[id] = identitydomain.Merchant{ID: id, Status: identitydomain.MerchantActive}
		keys.byPrefix[prefix] = identitydomain.APIKey{
			ID: uuid.New(), MerchantID: id, Prefix: prefix, Status: identitydomain.APIKeyActive,
		}
		return identitydomain.FormatToken(prefix, fakeAPIKeySecret)
	}
	h.tokenA = issue(h.merchA, "prefixa")
	h.tokenB = issue(h.merchB, "prefixb")

	auth := identityhttp.RequireAPIKey(identityapp.NewAuthenticateAPIKeyUseCase(keys, merchants, acceptAllHasher{}, stubClock{}))
	store := &memIdempotencyStore{records: map[string]*idempotency.Record{}}

	r := chi.NewRouter()
	httpapi.Mount(r, httpapi.Deps{
		CreatePaymentIntent: application.NewCreatePaymentIntentUseCase(h.repo, application.SystemClock{}, seqIDs{}),
		AuthorizePayment:    application.NewAuthorizePaymentUseCase(h.repo, h.acquirer, application.SystemClock{}, seqIDs{}),
		CapturePayment:      application.NewCapturePaymentUseCase(h.repo, application.SystemClock{}, seqIDs{}),
		Repo:                h.repo,
		AuthRequired:        auth,
		IdempotencyMW:       idempotency.Middleware(store, "create_payment_intent"),
	})
	h.router = r
	return h
}

func (h *harness) do(method, path, token, idemKey, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func (h *harness) createIntent(t *testing.T, token, key, body string) (string, *httptest.ResponseRecorder) {
	t.Helper()
	rec := h.do(http.MethodPost, "/v1/payment-intents", token, key, body)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusConflict {
		t.Fatalf("create: status %d body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID         string `json:"id"`
		MerchantID string `json:"merchant_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.ID, rec
}

func problemTitle(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var p struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem: %v body=%s", err, rec.Body)
	}
	return p.Title
}

// ---------- acceptance criterion 1: tenant isolation ----------

func TestForeignIntent_GetAuthorizeCapture_Return404AndLeaveIntentUnchanged(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id, _ := h.createIntent(t, h.tokenA, testIdempotencyKey, createBody)
	intentID := uuid.MustParse(id)
	before := h.repo.intents[intentID]
	outboxBefore := h.repo.outbox

	missing := uuid.NewString()
	cases := []struct{ name, method, suffix string }{
		{"get", http.MethodGet, ""},
		{"authorize", http.MethodPost, "/authorize"},
		{"capture", http.MethodPost, "/capture"},
	}
	for _, c := range cases {
		foreign := h.do(c.method, "/v1/payment-intents/"+id+c.suffix, h.tokenB, "", "")
		absent := h.do(c.method, "/v1/payment-intents/"+missing+c.suffix, h.tokenB, "", "")

		if foreign.Code != http.StatusNotFound {
			t.Errorf("%s foreign: status %d, want 404", c.name, foreign.Code)
		}
		if got := problemTitle(t, foreign); got != notFoundTitle {
			t.Errorf("%s foreign: title %q, want %q", c.name, got, notFoundTitle)
		}
		// Indistinguishable from an id that does not exist.
		if foreign.Code != absent.Code || foreign.Body.String() != absent.Body.String() {
			t.Errorf("%s: foreign response differs from missing-id response:\n%s\n%s", c.name, foreign.Body, absent.Body)
		}
	}

	if h.acquirer.calls != 0 {
		t.Errorf("acquirer called %d times on behalf of the wrong merchant", h.acquirer.calls)
	}
	if after := h.repo.intents[intentID]; after.Status != before.Status || after.AuthorizedAt != nil || after.CapturedAt != nil {
		t.Errorf("intent mutated: %+v", after)
	}
	if h.repo.outbox != outboxBefore {
		t.Errorf("outbox grew from %d to %d", outboxBefore, h.repo.outbox)
	}
}

func TestOwnIntent_GetAuthorizeCapture_Work(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id, _ := h.createIntent(t, h.tokenA, testIdempotencyKey, createBody)

	if rec := h.do(http.MethodGet, "/v1/payment-intents/"+id, h.tokenA, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodPost, "/v1/payment-intents/"+id+"/authorize", h.tokenA, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("authorize: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodPost, "/v1/payment-intents/"+id+"/capture", h.tokenA, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("capture: %d %s", rec.Code, rec.Body)
	}
}

// ---------- acceptance criteria 2 and 3: idempotency per merchant ----------

func TestIdempotency_SameKeySameBody_TwoMerchants_EachGetsOwnIntent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	idA, recA := h.createIntent(t, h.tokenA, sharedIdempotencyKey, createBody)
	idB, recB := h.createIntent(t, h.tokenB, sharedIdempotencyKey, createBody)

	if recA.Code != http.StatusCreated || recB.Code != http.StatusCreated {
		t.Fatalf("statuses A=%d B=%d, want 201 both", recA.Code, recB.Code)
	}
	if recB.Header().Get("Idempotent-Replay") != "" {
		t.Error("merchant B received a replay of merchant A's response")
	}
	if idA == idB {
		t.Fatalf("both merchants got intent %s", idA)
	}
	if got := h.repo.intents[uuid.MustParse(idB)].MerchantID; got != h.merchB {
		t.Errorf("B's intent belongs to %s, want %s", got, h.merchB)
	}
	if len(h.repo.intents) != 2 {
		t.Errorf("intents = %d, want 2", len(h.repo.intents))
	}
}

func TestIdempotency_SameKeyDifferentBody_OtherMerchant_NoConflict(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.createIntent(t, h.tokenA, sharedIdempotencyKey, createBody)

	rec := h.do(http.MethodPost, "/v1/payment-intents", h.tokenB, sharedIdempotencyKey, otherBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201 (keys are per merchant); body=%s", rec.Code, rec.Body)
	}
}

func TestIdempotency_SameMerchant_SameKeySameBody_ReplaysStoredResponse(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id, first := h.createIntent(t, h.tokenA, testIdempotencyKey, createBody)

	second := h.do(http.MethodPost, "/v1/payment-intents", h.tokenA, testIdempotencyKey, createBody)

	if second.Code != http.StatusCreated || second.Body.String() != first.Body.String() {
		t.Fatalf("replay differs: %d %s vs %s", second.Code, second.Body, first.Body)
	}
	if second.Header().Get("Idempotent-Replay") != "true" {
		t.Error("missing Idempotent-Replay header")
	}
	if len(h.repo.intents) != 1 {
		t.Errorf("intents = %d, want 1 (id %s)", len(h.repo.intents), id)
	}
}

func TestIdempotency_SameMerchant_SameKeyDifferentBody_Returns409(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.createIntent(t, h.tokenA, testIdempotencyKey, createBody)

	rec := h.do(http.MethodPost, "/v1/payment-intents", h.tokenA, testIdempotencyKey, otherBody)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409; body=%s", rec.Code, rec.Body)
	}
	if len(h.repo.intents) != 1 {
		t.Errorf("intents = %d, want 1", len(h.repo.intents))
	}
}

func TestCreate_WithoutCredentials_Returns401(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/v1/payment-intents", "garbage", testIdempotencyKey, createBody)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
}

// ---------- D3, D4: declines and lost races ----------

// H9: a lost version race answers 409 concurrent_update.
func TestAuthorize_LostRace_Returns409ConcurrentUpdate(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id, _ := h.createIntent(t, h.tokenA, testIdempotencyKey, createBody)
	h.repo.updateErr = domain.ErrConcurrentUpdate

	rec := h.do(http.MethodPost, "/v1/payment-intents/"+id+"/authorize", h.tokenA, "", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409; body=%s", rec.Code, rec.Body)
	}
	if got := problemTitle(t, rec); got != "concurrent_update" {
		t.Errorf("title %q, want concurrent_update", got)
	}
}

// The contract answers a decline with 200 and the failed intent.
func TestAuthorize_Declined_Returns200Failed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.acquirer.decline = true
	id, _ := h.createIntent(t, h.tokenA, testIdempotencyKey, createBody)

	rec := h.do(http.MethodPost, "/v1/payment-intents/"+id+"/authorize", h.tokenA, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Status != string(domain.PaymentIntentFailed) {
		t.Errorf("status field = %q err=%v, want failed", out.Status, err)
	}
}

func TestAuthorize_Captured_Returns409InvalidTransition(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id, _ := h.createIntent(t, h.tokenA, testIdempotencyKey, createBody)
	h.do(http.MethodPost, "/v1/payment-intents/"+id+"/authorize", h.tokenA, "", "")
	h.do(http.MethodPost, "/v1/payment-intents/"+id+"/capture", h.tokenA, "", "")
	calls := h.acquirer.calls

	rec := h.do(http.MethodPost, "/v1/payment-intents/"+id+"/authorize", h.tokenA, "", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409; body=%s", rec.Code, rec.Body)
	}
	if got := problemTitle(t, rec); got != "invalid_status_transition" {
		t.Errorf("title %q, want invalid_status_transition", got)
	}
	if h.acquirer.calls != calls {
		t.Error("acquirer called for a captured intent")
	}
}

func TestCreate_MissingIdempotencyKey_Returns400(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := h.do(http.MethodPost, "/v1/payment-intents", h.tokenA, "", createBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}
