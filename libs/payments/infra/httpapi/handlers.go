// Package httpapi expõe Payments como handlers chi.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	contracts "github.com/iris-gate/iris-gate/libs/contracts/generated/go/payments"
	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

// Deps agrupa o que os handlers consomem.
type Deps struct {
	CreatePaymentIntent *application.CreatePaymentIntentUseCase
	AuthorizePayment    *application.AuthorizePaymentUseCase
	CapturePayment      *application.CapturePaymentUseCase
	Repo                application.PaymentIntentRepo

	// AuthRequired/IdempotencyMW são plugados pelo composer (apps/api).
	// Mantemos como middlewares opacos pra evitar dependência circular.
	AuthRequired  func(http.Handler) http.Handler
	IdempotencyMW func(http.Handler) http.Handler
}

// Mount registra rotas em `r`.
//
//   GET  /v1/payment-intents               (auth)            — list
//   POST /v1/payment-intents               (auth + idempotency) — create
//   GET  /v1/payment-intents/{id}          (auth)            — detail
//   POST /v1/payment-intents/{id}/authorize (auth)
//   POST /v1/payment-intents/{id}/capture   (auth)
//
// No-op se CreatePaymentIntent / AuthRequired estiverem nil.
func Mount(r chi.Router, d Deps) {
	if d.CreatePaymentIntent == nil || d.AuthRequired == nil {
		return
	}

	// POST /v1/payment-intents — exige idempotency middleware.
	if d.IdempotencyMW != nil {
		r.Group(func(r chi.Router) {
			r.Use(d.AuthRequired)
			r.Use(d.IdempotencyMW)
			r.Post("/v1/payment-intents", handleCreatePaymentIntent(d.CreatePaymentIntent))
		})
	}

	// /authorize, /capture, GET list, GET detail — naturalmente idempotentes ou
	// puramente leitura.
	r.Group(func(r chi.Router) {
		r.Use(d.AuthRequired)
		if d.AuthorizePayment != nil {
			r.Post("/v1/payment-intents/{id}/authorize", handleAuthorize(d.AuthorizePayment))
		}
		if d.CapturePayment != nil {
			r.Post("/v1/payment-intents/{id}/capture", handleCapture(d.CapturePayment))
		}
		if d.Repo != nil {
			r.Get("/v1/payment-intents", handleListPaymentIntents(d.Repo))
			r.Get("/v1/payment-intents/{id}", handleGetPaymentIntent(d.Repo))
		}
	})
}

func handleListPaymentIntents(repo application.PaymentIntentRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchant, ok := identityhttp.MerchantFromContext(r.Context())
		if !ok {
			problem(w, r, http.StatusUnauthorized, "no_principal", "merchant ausente")
			return
		}
		limit := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		items, err := repo.ListByMerchant(r.Context(), merchant.ID, limit)
		if err != nil {
			mapError(w, r, err)
			return
		}
		out := make([]contracts.PaymentIntent, 0, len(items))
		for _, p := range items {
			out = append(out, toContract(p))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func handleGetPaymentIntent(repo application.PaymentIntentRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		merchant, ok := identityhttp.MerchantFromContext(r.Context())
		if !ok {
			problem(w, r, http.StatusUnauthorized, "no_principal", "merchant ausente")
			return
		}
		id, err := parseIDParam(r, "id")
		if err != nil {
			problem(w, r, http.StatusBadRequest, "invalid_id", err.Error())
			return
		}
		pi, err := repo.GetByID(r.Context(), id)
		if err != nil {
			mapError(w, r, err)
			return
		}
		if pi.MerchantID != merchant.ID {
			problem(w, r, http.StatusNotFound, "not_found", "payment intent não encontrado")
			return
		}
		writeJSON(w, http.StatusOK, toContract(pi))
	}
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

func handleAuthorize(uc *application.AuthorizePaymentUseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r, "id")
		if err != nil {
			problem(w, r, http.StatusBadRequest, "invalid_id", err.Error())
			return
		}
		pi, err := uc.Execute(r.Context(), id)
		if err != nil {
			mapError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toContract(pi))
	}
}

func handleCapture(uc *application.CapturePaymentUseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r, "id")
		if err != nil {
			problem(w, r, http.StatusBadRequest, "invalid_id", err.Error())
			return
		}
		pi, err := uc.Execute(r.Context(), id)
		if err != nil {
			mapError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toContract(pi))
	}
}

func parseIDParam(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, name))
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
