package idempotency_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/infra/idempotency"
)

// The replay, conflict and per-merchant behavior needs an authenticated
// principal, which only identity's RequireAPIKey can inject. Those cases are
// exercised end to end in libs/payments/infra/httpapi.

type unreachableStore struct{}

func (unreachableStore) Existing(context.Context, uuid.UUID, string, string) (*idempotency.Record, error) {
	panic("store must not be consulted without a principal")
}

func (unreachableStore) Save(context.Context, uuid.UUID, string, string, []byte, int, []byte) (bool, error) {
	panic("store must not be written without a principal")
}

func TestMiddleware_WithoutPrincipal_Returns401AndSkipsHandler(t *testing.T) {
	t.Parallel()
	var calls int32
	mw := idempotency.Middleware(unreachableStore{}, "test")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", "k1")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if calls != 0 {
		t.Fatalf("handler called %d times, want 0", calls)
	}
}
