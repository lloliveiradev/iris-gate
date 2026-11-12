// Package idempotency is an HTTP middleware backed by a Store (Postgres in
// production).
//
// Behavior (Stripe-style idempotency), per (merchant, scope, key):
//
//  1. No authenticated merchant → 401.
//  2. No header → 400.
//  3. Header present, new key → run the request and store (status, body)
//     before returning. Later requests with the same key and body replay the
//     stored response without re-running the handler.
//  4. Same key, different body → 409 (request_hash mismatch).
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
)

// Store is the port the middleware consumes (Postgres in production).
type Store interface {
	Existing(ctx context.Context, merchantID uuid.UUID, scope, key string) (*Record, error)
	Save(ctx context.Context, merchantID uuid.UUID, scope, key string, requestHash []byte, status int, body []byte) (alreadyExists bool, err error)
}

type Record struct {
	RequestHash    []byte
	ResponseStatus int
	ResponseBody   []byte
}

// Middleware returns an http.Handler middleware for the given route `scope`
// (e.g. "create_payment_intent"). Records are keyed by (merchant, scope, key):
// the merchant comes from the authenticated principal, so it must be mounted
// after the authentication middleware. Without a principal it answers 401.
func Middleware(store Store, scope string) func(http.Handler) http.Handler {
	const header = "Idempotency-Key"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			merchant, ok := identityhttp.MerchantFromContext(r.Context())
			if !ok {
				writeProblem(w, http.StatusUnauthorized, "no_principal", "authenticated merchant required")
				return
			}
			merchantID := merchant.ID

			key := r.Header.Get(header)
			if key == "" {
				writeProblem(w, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header required")
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeProblem(w, http.StatusBadRequest, "invalid_body", err.Error())
				return
			}
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))

			hash := sha256.Sum256(body)

			if existing, err := store.Existing(r.Context(), merchantID, scope, key); err != nil {
				slog.LogAttrs(r.Context(), slog.LevelError, "idempotency.lookup", slog.Any("err", err))
				writeProblem(w, http.StatusInternalServerError, "internal", "idempotency lookup failed")
				return
			} else if existing != nil {
				if !bytes.Equal(existing.RequestHash, hash[:]) {
					writeProblem(w, http.StatusConflict, "idempotency_conflict", "key reused with a different body")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Idempotent-Replay", "true")
				w.WriteHeader(existing.ResponseStatus)
				_, _ = w.Write(existing.ResponseBody)
				return
			}

			rec := &recorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			if rec.status >= 200 && rec.status < 300 {
				if _, err := store.Save(r.Context(), merchantID, scope, key, hash[:], rec.status, rec.body.Bytes()); err != nil {
					slog.LogAttrs(r.Context(), slog.LevelError, "idempotency.save", slog.Any("err", err))
				}
			}
		})
	}
}

type recorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *recorder) WriteHeader(s int) {
	r.status = s
	r.ResponseWriter.WriteHeader(s)
}

func (r *recorder) Write(p []byte) (int, error) {
	r.body.Write(p)
	return r.ResponseWriter.Write(p)
}

func writeProblem(w http.ResponseWriter, code int, title, detail string) {
	d := detail
	body := map[string]any{
		"type":   "about:blank",
		"title":  title,
		"status": code,
		"detail": d,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
