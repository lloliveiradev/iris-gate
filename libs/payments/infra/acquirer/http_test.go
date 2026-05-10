package acquirer_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/infra/acquirer"
)

func TestHTTPAcquirer_Approved(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/authorize" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("auth header = %q", got)
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

	a := acquirer.NewHTTP(srv.URL, "secret")
	out, err := a.Authorize(context.Background(), application.AuthorizeInput{
		IntentID:    uuid.New(),
		AmountCents: 1234,
		Currency:    "BRL",
		CardToken:   "tok_x",
	})
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

	a := acquirer.NewHTTP(srv.URL, "")
	out, err := a.Authorize(context.Background(), application.AuthorizeInput{
		IntentID:    uuid.New(),
		AmountCents: 100,
		Currency:    "BRL",
		CardToken:   "tok_x",
	})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if out.Approved {
		t.Errorf("approved deveria ser false, got %+v", out)
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

	a := acquirer.NewHTTP(srv.URL, "")
	_, err := a.Authorize(context.Background(), application.AuthorizeInput{
		IntentID:    uuid.New(),
		AmountCents: 100,
		Currency:    "BRL",
		CardToken:   "tok_x",
	})
	var httpErr *acquirer.ErrAcquirerHTTP
	if !errors.As(err, &httpErr) {
		t.Fatalf("err = %v, want ErrAcquirerHTTP", err)
	}
	if httpErr.Status != http.StatusTooManyRequests {
		t.Errorf("status = %d", httpErr.Status)
	}
}
