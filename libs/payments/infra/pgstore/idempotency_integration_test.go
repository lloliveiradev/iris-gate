//go:build integration

package pgstore_test

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
	"github.com/jackc/pgx/v5/pgxpool"

	identityapp "github.com/iris-gate/iris-gate/libs/identity/application"
	identitydomain "github.com/iris-gate/iris-gate/libs/identity/domain"
	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/infra/httpapi"
	"github.com/iris-gate/iris-gate/libs/payments/infra/idempotency"
	"github.com/iris-gate/iris-gate/libs/payments/infra/pgstore"
	"github.com/iris-gate/iris-gate/libs/payments/testfixtures"
)

// concurrentFirstRequests is how many first requests with one idempotency
// key race in TestIntegration_RLS_IdempotencyConcurrentFirstRequestsCreateOneIntent.
const concurrentFirstRequests = 24

// Fake values of the concurrency test. fakeAPIKeyPrefix and fakeAPIKeySecret
// form the bearer token; acceptAnySecret accepts any secret.
const (
	fakeAPIKeyPrefix = "prefixrace"
	fakeAPIKeySecret = "secret"
	createIntentPath = "/v1/payment-intents"
	createIntentBody = `{"amount":{"amount_cents":1000,"currency":"BRL"},"card_token":"` + testfixtures.TokenApproved + `"}`
)

// Wire values the idempotency middleware answers with.
const (
	replayHeader    = "Idempotent-Replay"
	inProgressTitle = "idempotency_in_progress"
)

const (
	countMerchantIntentsSQL = `SELECT count(*) FROM payment_intents WHERE merchant_id = $1`
	countOutboxByKeySQL     = `SELECT count(*) FROM outbox_messages WHERE topic = $1 AND msg_key = $2`
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
// merchant, and the bearer token that authenticates as it. The merchant row
// itself must exist (seedMerchant): intents and idempotency records reference
// it.
func authenticateAs(merchantID uuid.UUID) (auth func(http.Handler) http.Handler, token string) {
	key := identitydomain.APIKey{
		ID: uuid.New(), MerchantID: merchantID, Prefix: fakeAPIKeyPrefix, Status: identitydomain.APIKeyActive,
	}
	merchant := identitydomain.Merchant{ID: merchantID, Status: identitydomain.MerchantActive}
	uc := identityapp.NewAuthenticateAPIKeyUseCase(
		oneAPIKey{key: key}, oneMerchant{merchant: merchant}, acceptAnySecret{}, identityapp.SystemClock{})
	return identityhttp.RequireAPIKey(uc), identitydomain.FormatToken(fakeAPIKeyPrefix, fakeAPIKeySecret)
}

// ---------- helpers ----------

// newCreateIntentRouter mounts the create route as apps/api does (real
// authentication middleware, idempotency middleware, handler, use case and
// Postgres adapters), on db.
func newCreateIntentRouter(db *pgxpool.Pool, auth func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()
	httpapi.Mount(r, httpapi.Deps{
		CreatePaymentIntent: application.NewCreatePaymentIntentUseCase(
			pgstore.New(db), application.SystemClock{}, application.UUIDGen{}),
		AuthRequired:  auth,
		IdempotencyMW: idempotency.Middleware(pgstore.NewIdempotencyStore(db), idempotencyScope),
	})
	return r
}

func postCreateIntent(h http.Handler, token, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, createIntentPath, strings.NewReader(createIntentBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// postConcurrently sends n identical create requests with one key at once.
func postConcurrently(h http.Handler, token, key string, n int) []*httptest.ResponseRecorder {
	out := make([]*httptest.ResponseRecorder, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			<-start
			out[i] = postCreateIntent(h, token, key)
		})
	}
	close(start)
	wg.Wait()
	return out
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func isInProgress(rec *httptest.ResponseRecorder) bool {
	var problem struct {
		Title string `json:"title"`
	}
	return rec.Code == http.StatusConflict &&
		json.Unmarshal(rec.Body.Bytes(), &problem) == nil &&
		problem.Title == inProgressTitle
}

// ---------- test ----------

// TestIntegration_RLS_IdempotencyConcurrentFirstRequestsCreateOneIntent fires
// concurrentFirstRequests first requests with one key at the real middleware
// and Postgres store, as iris_app. Exactly one creates the intent and its
// created.v1 outbox row; every other request gets the identical stored
// response or 409 idempotency_in_progress.
func TestIntegration_RLS_IdempotencyConcurrentFirstRequestsCreateOneIntent(t *testing.T) {
	pool := newPool(t)
	appPool := newAppPool(t)
	merchant := seedMerchant(t, pool)
	auth, token := authenticateAs(merchant)
	router := newCreateIntentRouter(appPool, auth)
	key := uuid.NewString()

	responses := postConcurrently(router, token, key, concurrentFirstRequests)

	var original *httptest.ResponseRecorder
	var replays, inProgress int
	for i, rec := range responses {
		switch {
		case rec.Code == http.StatusCreated && rec.Header().Get(replayHeader) == "":
			if original != nil {
				t.Fatalf("request %d ran the handler too: %s", i, rec.Body.String())
			}
			original = rec
		case rec.Code == http.StatusCreated && rec.Header().Get(replayHeader) == "true":
			replays++
		case isInProgress(rec):
			inProgress++
		default:
			t.Fatalf("request %d: %d %s, want 201 or 409 %s", i, rec.Code, rec.Body.String(), inProgressTitle)
		}
	}
	if original == nil {
		t.Fatal("no request created the intent")
	}
	t.Logf("%d requests: 1 created, %d replayed, %d answered in progress", concurrentFirstRequests, replays, inProgress)

	for i, rec := range responses {
		if rec.Code == http.StatusCreated && rec.Body.String() != original.Body.String() {
			t.Errorf("request %d: body %s, want the stored response %s", i, rec.Body.String(), original.Body.String())
		}
	}
	replay := postCreateIntent(router, token, key)
	if replay.Code != http.StatusCreated || replay.Header().Get(replayHeader) != "true" || replay.Body.String() != original.Body.String() {
		t.Errorf("retry after the race = %d %s replay=%q, want the stored response replayed",
			replay.Code, replay.Body.String(), replay.Header().Get(replayHeader))
	}

	if n := countRows(t, pool, countMerchantIntentsSQL, merchant); n != 1 {
		t.Errorf("payment intents = %d, want 1", n)
	}
	if n := countRows(t, pool, countOutboxByKeySQL, application.TopicPaymentIntentCreatedV1, merchant.String()); n != 1 {
		t.Errorf("created.v1 outbox rows = %d, want 1", n)
	}
}
