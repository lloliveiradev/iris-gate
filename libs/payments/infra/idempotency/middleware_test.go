package idempotency_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/iris-gate/iris-gate/libs/payments/infra/idempotency"
)

// fakeStore é uma store in-memory implementando idempotency.Store.
type fakeStore struct {
	mu      sync.Mutex
	records map[string]*idempotency.Record // chave: scope + "|" + key
}

func newFakeStore() *fakeStore { return &fakeStore{records: map[string]*idempotency.Record{}} }

func (s *fakeStore) Existing(_ context.Context, scope, key string) (*idempotency.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[scope+"|"+key]
	if !ok {
		return nil, nil
	}
	return rec, nil
}

func (s *fakeStore) Save(_ context.Context, scope, key string, hash []byte, status int, body []byte) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := scope + "|" + key
	if _, ok := s.records[k]; ok {
		return true, nil
	}
	s.records[k] = &idempotency.Record{RequestHash: hash, ResponseStatus: status, ResponseBody: body}
	return false, nil
}

func TestMiddleware_FirstCall_ExecutesHandler(t *testing.T) {
	t.Parallel()
	var calls int32
	store := newFakeStore()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	})
	mw := idempotency.Middleware(store, "test")(handler)

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
	req.Header.Set("Idempotency-Key", "k1")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	if calls != 1 {
		t.Fatalf("handler chamado %d vezes, want 1", calls)
	}
}

func TestMiddleware_SecondCallSameKey_ReplaysResponse(t *testing.T) {
	t.Parallel()
	var calls int32
	store := newFakeStore()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	})
	mw := idempotency.Middleware(store, "test")(handler)

	doRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
		req.Header.Set("Idempotency-Key", "k1")
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		return rec
	}

	first := doRequest()
	second := doRequest()

	if calls != 1 {
		t.Fatalf("handler chamado %d vezes, want 1 (idempotência falhou)", calls)
	}
	if first.Code != second.Code {
		t.Errorf("status divergente: %d vs %d", first.Code, second.Code)
	}
	if first.Body.String() != second.Body.String() {
		t.Errorf("body divergente: %q vs %q", first.Body.String(), second.Body.String())
	}
	if h := second.Header().Get("Idempotent-Replay"); h != "true" {
		t.Errorf("Idempotent-Replay header = %q, want 'true'", h)
	}
}

func TestMiddleware_SameKeyDifferentBody_Returns409(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.Copy(w, r.Body)
	})
	mw := idempotency.Middleware(store, "test")(handler)

	first := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
	first.Header.Set("Idempotency-Key", "k1")
	r1 := httptest.NewRecorder()
	mw.ServeHTTP(r1, first)
	if r1.Code != http.StatusCreated {
		t.Fatalf("first call: %d", r1.Code)
	}

	second := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":2}`))
	second.Header.Set("Idempotency-Key", "k1")
	r2 := httptest.NewRecorder()
	mw.ServeHTTP(r2, second)
	if r2.Code != http.StatusConflict {
		t.Fatalf("second call: %d, want 409", r2.Code)
	}
}

func TestMiddleware_MissingHeader_Returns400(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	mw := idempotency.Middleware(store, "test")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
