package acquirer_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/infra/acquirer"
	"github.com/iris-gate/iris-gate/libs/payments/testfixtures"
)

// authorizeInput is a valid request keyed, like the use case does, by the intent id.
func authorizeInput(amountCents int64, cardToken string) application.AuthorizeInput {
	id := uuid.New()
	return application.AuthorizeInput{
		IntentID:       id,
		AmountCents:    amountCents,
		Currency:       "BRL",
		CardToken:      cardToken,
		IdempotencyKey: id.String(),
	}
}

// A2: the intent id travels as the Idempotency-Key header.
func TestHTTPAcquirer_Approved_SendsIdempotencyKey(t *testing.T) {
	t.Parallel()

	in := authorizeInput(1234, testfixtures.TokenApproved)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/authorize" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testfixtures.AcquirerBearer {
			t.Errorf("auth header = %q", got)
		}
		if got := r.Header.Get(acquirer.IdempotencyKeyHeader); got != in.IntentID.String() {
			t.Errorf("idempotency key header = %q, want %s", got, in.IntentID)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"amount_cents":1234`) {
			t.Errorf("body missing amount: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"acquirer_ref": "acq-xyz",
			"approved":     true,
			"reason":       "ok",
		})
	}))
	defer srv.Close()

	out, err := acquirer.NewHTTP(srv.URL, testfixtures.AcquirerBearer).Authorize(context.Background(), in)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !out.Approved || out.AcquirerRef != "acq-xyz" {
		t.Errorf("got %+v", out)
	}
}

func TestHTTPAcquirer_Rejected(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"acquirer_ref": "acq-rej",
			"approved":     false,
			"reason":       "insufficient_funds",
		})
	}))
	defer srv.Close()

	out, err := acquirer.NewHTTP(srv.URL, "").Authorize(context.Background(), authorizeInput(100, testfixtures.TokenApproved))
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if out.Approved {
		t.Errorf("approved should be false, got %+v", out)
	}
	if out.Reason != "insufficient_funds" {
		t.Errorf("reason = %q", out.Reason)
	}
}

func TestHTTPAcquirer_Non2xx_ReturnsErrAcquirerHTTP(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "downstream rate limit", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := acquirer.NewHTTP(srv.URL, "").Authorize(context.Background(), authorizeInput(100, testfixtures.TokenApproved))
	var httpErr *acquirer.ErrAcquirerHTTP
	if !errors.As(err, &httpErr) {
		t.Fatalf("err = %v, want ErrAcquirerHTTP", err)
	}
	if httpErr.Status != http.StatusTooManyRequests {
		t.Errorf("status = %d", httpErr.Status)
	}
}

func TestHTTPAcquirer_MissingIdempotencyKey_DoesNotCall(t *testing.T) {
	t.Parallel()

	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called.Store(true) }))
	defer srv.Close()

	in := authorizeInput(100, testfixtures.TokenApproved)
	in.IdempotencyKey = ""
	if _, err := acquirer.NewHTTP(srv.URL, "").Authorize(context.Background(), in); !errors.Is(err, acquirer.ErrIdempotencyKeyRequired) {
		t.Fatalf("err = %v, want ErrIdempotencyKeyRequired", err)
	}
	if called.Load() {
		t.Error("backend called without an idempotency key")
	}
}
