// Package httpapi expõe Identity como handlers chi.
//
// Toda a tradução entre DTOs gerados pelo OpenAPI (libs/contracts/...) e
// objetos de domínio mora aqui. apps/api só compõe.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	contracts "github.com/iris-gate/iris-gate/libs/contracts/generated/go/identity"
	"github.com/iris-gate/iris-gate/libs/identity/application"
	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

// Deps agrupa os use cases consumidos pelos handlers.
type Deps struct {
	CreateMerchant *application.CreateMerchantUseCase
	IssueAPIKey    *application.IssueAPIKeyUseCase
	RevokeAPIKey   *application.RevokeAPIKeyUseCase
	Authenticate   *application.AuthenticateAPIKeyUseCase
}

// Mount registra as rotas de Identity em `r`.
//
// /v1/merchants                   POST  (público — bootstrap)
// /v1/api-keys                    POST  (público — bootstrap)
// /v1/api-keys/{id}/revoke        POST  (público — bootstrap)
// /v1/me                          GET   (protegido por RequireAPIKey)
//
// Hardening (require admin) entra na Fase 7.
func Mount(r chi.Router, d Deps) {
	// No-op se nenhum use case foi cabeado (apps/api podem testar healthz
	// sem precisar trazer Identity junto).
	if d.CreateMerchant == nil && d.IssueAPIKey == nil && d.RevokeAPIKey == nil && d.Authenticate == nil {
		return
	}
	r.Post("/v1/merchants", handleCreateMerchant(d.CreateMerchant))
	r.Post("/v1/api-keys", handleIssueAPIKey(d.IssueAPIKey))
	r.Post("/v1/api-keys/{id}/revoke", handleRevokeAPIKey(d.RevokeAPIKey))

	r.Group(func(r chi.Router) {
		r.Use(RequireAPIKey(d.Authenticate))
		r.Get("/v1/me", handleMe)
	})
}

func handleCreateMerchant(uc *application.CreateMerchantUseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body contracts.CreateMerchantRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			problem(w, r, http.StatusBadRequest, "invalid_body", "body inválido")
			return
		}
		m, err := uc.Execute(r.Context(), application.CreateMerchantInput{
			LegalName:    body.LegalName,
			Document:     body.Document,
			ContactEmail: string(body.ContactEmail),
		})
		if err != nil {
			mapError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, toContractMerchant(m))
	}
}

func handleIssueAPIKey(uc *application.IssueAPIKeyUseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body contracts.IssueApiKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			problem(w, r, http.StatusBadRequest, "invalid_body", "body inválido")
			return
		}
		merchantID, err := uuid.Parse(body.MerchantId.String())
		if err != nil {
			problem(w, r, http.StatusBadRequest, "invalid_merchant_id", err.Error())
			return
		}
		scopes := []string{}
		if body.Scopes != nil {
			scopes = *body.Scopes
		}
		out, err := uc.Execute(r.Context(), application.IssueAPIKeyInput{
			MerchantID: merchantID,
			Scopes:     scopes,
		})
		if err != nil {
			mapError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, contracts.IssueApiKeyResponse{
			ApiKey: toContractAPIKey(out.APIKey),
			Secret: out.Token,
		})
	}
}

func handleRevokeAPIKey(uc *application.RevokeAPIKeyUseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := chi.URLParam(r, "id")
		id, err := uuid.Parse(raw)
		if err != nil {
			problem(w, r, http.StatusBadRequest, "invalid_id", err.Error())
			return
		}
		k, err := uc.Execute(r.Context(), id)
		if err != nil {
			mapError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toContractAPIKey(k))
	}
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	m, ok := MerchantFromContext(r.Context())
	if !ok {
		problem(w, r, http.StatusUnauthorized, "no_principal", "merchant ausente do contexto")
		return
	}
	writeJSON(w, http.StatusOK, toContractMerchant(m))
}

// ---------- mapeamento DTO ↔ domain ----------

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

func toContractAPIKey(k domain.APIKey) contracts.ApiKey {
	scopes := k.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return contracts.ApiKey{
		Id:         openapi_types.UUID(k.ID),
		MerchantId: openapi_types.UUID(k.MerchantID),
		Prefix:     k.Prefix,
		Status:     contracts.ApiKeyStatus(k.Status),
		Scopes:     scopes,
		CreatedAt:  k.CreatedAt,
		LastUsedAt: k.LastUsedAt,
		RevokedAt:  k.RevokedAt,
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

// mapError traduz erros de domínio para responses HTTP problem+json.
func mapError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		problem(w, r, http.StatusBadRequest, "validation", err.Error())
	case errors.Is(err, domain.ErrDocumentExists):
		problem(w, r, http.StatusConflict, "document_exists", err.Error())
	case errors.Is(err, domain.ErrMerchantNotFound), errors.Is(err, domain.ErrAPIKeyNotFound):
		problem(w, r, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrAPIKeyInvalid):
		problem(w, r, http.StatusUnauthorized, "invalid_credentials", err.Error())
	case errors.Is(err, domain.ErrAPIKeyRevoked):
		problem(w, r, http.StatusForbidden, "revoked", err.Error())
	default:
		slog.LogAttrs(r.Context(), slog.LevelError, "http.unhandled", slog.Any("err", err))
		problem(w, r, http.StatusInternalServerError, "internal", "internal error")
	}
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
