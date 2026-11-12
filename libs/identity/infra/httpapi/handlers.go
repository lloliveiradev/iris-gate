// Package httpapi exposes Identity as chi handlers.
//
// The translation between the OpenAPI-generated DTOs (libs/contracts/...) and
// domain objects lives here. apps/api only composes.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	contracts "github.com/iris-gate/iris-gate/libs/contracts/generated/go/identity"
	"github.com/iris-gate/iris-gate/libs/identity/application"
	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

// Deps groups the use cases consumed by the handlers. Merchant and API key
// management are operator tasks (apps/api/cmd/irisctl), not HTTP routes.
type Deps struct {
	Authenticate *application.AuthenticateAPIKeyUseCase
}

// Mount registers the Identity routes on r.
//
//	/v1/me    GET  (protected by RequireAPIKey)
//
// There is no route to create merchants or to issue or revoke API keys (D8).
func Mount(r chi.Router, d Deps) {
	// No-op when nothing is wired, so apps/api can serve healthz without Identity.
	if d.Authenticate == nil {
		return
	}

	r.Group(func(r chi.Router) {
		r.Use(RequireAPIKey(d.Authenticate))
		r.Get("/v1/me", handleMe)
	})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	m, ok := MerchantFromContext(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "no_principal", "merchant missing from context")
		return
	}
	writeJSON(w, http.StatusOK, toContractMerchant(m))
}

// ---------- DTO mapping ----------

func toContractMerchant(m domain.Merchant) contracts.Merchant {
	return contracts.Merchant{
		Id:           openapi_types.UUID(m.ID),
		LegalName:    m.LegalName,
		Document:     m.Document,
		ContactEmail: openapi_types.Email(m.ContactEmail),
		Status:       contracts.MerchantStatus(m.Status),
		CreatedAt:    m.CreatedAt,
	}
}

// ---------- response helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func problem(w http.ResponseWriter, r *http.Request, code int, title, detail string) {
	d := detail
	p := contracts.Problem{
		Type:   "about:blank",
		Title:  title,
		Status: code,
		Detail: &d,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(p)
	slog.LogAttrs(r.Context(), slog.LevelInfo, "http.problem",
		slog.Int("status", code),
		slog.String("title", title),
		slog.String("detail", detail),
		slog.String("path", r.URL.Path),
	)
}

// ---------- ctx helpers ----------

type ctxKey int

const (
	ctxMerchant ctxKey = iota
	ctxAPIKey
)

func contextWithMerchant(ctx context.Context, m domain.Merchant, k domain.APIKey) context.Context {
	ctx = context.WithValue(ctx, ctxMerchant, m)
	ctx = context.WithValue(ctx, ctxAPIKey, k)
	return ctx
}

// MerchantFromContext devolve o merchant autenticado pelo middleware RequireAPIKey.
func MerchantFromContext(ctx context.Context) (domain.Merchant, bool) {
	m, ok := ctx.Value(ctxMerchant).(domain.Merchant)
	return m, ok
}

// APIKeyFromContext devolve a api key usada pra autenticar (audit, scopes, rate-limit).
func APIKeyFromContext(ctx context.Context) (domain.APIKey, bool) {
	k, ok := ctx.Value(ctxAPIKey).(domain.APIKey)
	return k, ok
}
