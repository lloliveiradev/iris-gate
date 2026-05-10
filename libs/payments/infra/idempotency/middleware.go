// Package idempotency é um middleware HTTP backed por Postgres.
//
// Comportamento (alinha com Stripe-style idempotency):
//
//   1. Sem header → 400.
//   2. Header presente, key nova → executa request, persiste (status, body)
//      antes de devolver. Próximas requests com mesma key + body devolvem
//      a mesma resposta sem reexecução.
//   3. Mesma key, body diferente → 409 (request_hash não bate).
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// Store é o port consumido pelo middleware (Postgres-backed em prod).
type Store interface {
	Existing(ctx context.Context, scope, key string) (*Record, error)
	Save(ctx context.Context, scope, key string, requestHash []byte, status int, body []byte) (alreadyExists bool, err error)
}

type Record struct {
	RequestHash    []byte
	ResponseStatus int
	ResponseBody   []byte
}

// Middleware retorna um http.Handler middleware com o `scope` dado (ex.:
// "create_payment_intent"). Header configurável; default `Idempotency-Key`.
func Middleware(store Store, scope string) func(http.Handler) http.Handler {
	const header = "Idempotency-Key"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(header)
			if key == "" {
				writeProblem(w, http.StatusBadRequest, "missing_idempotency_key", "header Idempotency-Key obrigatório")
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

			if existing, err := store.Existing(r.Context(), scope, key); err != nil {
				slog.LogAttrs(r.Context(), slog.LevelError, "idempotency.lookup", slog.Any("err", err))
				writeProblem(w, http.StatusInternalServerError, "internal", "idempotency lookup falhou")
				return
			} else if existing != nil {
				if !bytes.Equal(existing.RequestHash, hash[:]) {
					writeProblem(w, http.StatusConflict, "idempotency_conflict", "key reutilizada com body diferente")
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
				if _, err := store.Save(r.Context(), scope, key, hash[:], rec.status, rec.body.Bytes()); err != nil {
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
