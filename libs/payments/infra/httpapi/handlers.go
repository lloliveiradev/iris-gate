// Package httpapi expõe Payments como handlers chi.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	contracts "github.com/iris-gate/iris-gate/libs/contracts/generated/go/payments"
	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

// Deps agrupa o que os handlers consomem.
type Deps struct {
	CreatePaymentIntent *application.CreatePaymentIntentUseCase

	// AuthRequired/IdempotencyMW são plugados pelo composer (apps/api).
	// Mantemos como middlewares opacos pra evitar dependência circular.
	AuthRequired   func(http.Handler) http.Handler
	IdempotencyMW  func(http.Handler) http.Handler
}

// Mount registra rotas em `r`.
//
//   POST /v1/payment-intents   (auth + idempotency)
//
// No-op se algum dep obrigatório for nil — útil em testes que só querem
// montar o resto do router sem trazer Payments junto.
func Mount(r chi.Router, d Deps) {
	if d.CreatePaymentIntent == nil || d.AuthRequired == nil || d.IdempotencyMW == nil {
		return
	}
	r.Group(func(r chi.Router) {
		r.Use(d.AuthRequired)
		r.Use(d.IdempotencyMW)
		r.Post("/v1/payment-intents", handleCreatePaymentIntent(d.CreatePaymentIntent))
	})
}

func handleCreatePaymentIntent(uc *application.CreatePaymentIntentUseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchant, ok := identityhttp.MerchantFromContext(r.Context())
		if !ok {
			problem(w, r, http.StatusUnauthorized, "no_principal", "merchant ausente")
			return
		}

		var body contracts.CreatePaymentIntentRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			problem(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		money, err := domain.NewMoney(body.Amount.AmountCents, string(body.Amount.Currency))
		if err != nil {
			problem(w, r, http.StatusBadRequest, "invalid_amount", err.Error())
			return
		}

		desc := ""
		if body.Description != nil {
			desc = *body.Description
		}

		pi, err := uc.Execute(r.Context(), application.CreatePaymentIntentInput{
			MerchantID:  merchant.ID,
			Amount:      money,
			CardToken:   body.CardToken,
			Description: desc,
		})
		if err != nil {
			mapError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, toContract(pi))
	}
}

// ---------- DTO mapping ----------

func toContract(p domain.PaymentIntent) contracts.PaymentIntent {
	out := contracts.PaymentIntent{
		Id:         openapi_types.UUID(p.ID),
		MerchantId: openapi_types.UUID(p.MerchantID),
		Amount: contracts.Money{
			AmountCents: p.Amount.AmountCents,
			Currency:    p.Amount.Currency,
		},
		Status:       contracts.PaymentIntentStatus(p.Status),
		CreatedAt:    p.CreatedAt,
		AuthorizedAt: p.AuthorizedAt,
		CapturedAt:   p.CapturedAt,
	}
	if p.CardToken != "" {
		ct := p.CardToken
		out.CardToken = &ct
	}
	if p.Description != "" {
		desc := p.Description
		out.Description = &desc
	}
	return out
}

// ---------- response helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func problem(w http.ResponseWriter, r *http.Request, code int, title, detail string) {
	d := detail
	p := contracts.Problem{Type: "about:blank", Title: title, Status: code, Detail: &d}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(p)
	slog.LogAttrs(r.Context(), slog.LevelInfo, "http.problem",
		slog.Int("status", code), slog.String("title", title),
		slog.String("path", r.URL.Path),
	)
}

func mapError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		problem(w, r, http.StatusBadRequest, "validation", err.Error())
	case errors.Is(err, domain.ErrIdempotencyConflict):
		problem(w, r, http.StatusConflict, "idempotency_conflict", err.Error())
	case errors.Is(err, domain.ErrPaymentIntentNotFound):
		problem(w, r, http.StatusNotFound, "not_found", err.Error())
	default:
		slog.LogAttrs(r.Context(), slog.LevelError, "http.unhandled", slog.Any("err", err))
		problem(w, r, http.StatusInternalServerError, "internal", "internal error")
	}
}
