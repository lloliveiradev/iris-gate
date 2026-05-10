package domain

import (
	"time"

	"github.com/google/uuid"
)

// PaymentIntentCreatedV1 é o payload do evento `payments.payment_intent.created.v1`
// publicado via Outbox pattern (ADR-0002) quando um PaymentIntent é criado.
type PaymentIntentCreatedV1 struct {
	EventID         uuid.UUID `json:"event_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	PaymentIntentID uuid.UUID `json:"payment_intent_id"`
	MerchantID      uuid.UUID `json:"merchant_id"`
	AmountCents     int64     `json:"amount_cents"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
}

// PaymentIntentAuthorizedV1 disparado quando o adquirente aprova.
type PaymentIntentAuthorizedV1 struct {
	EventID         uuid.UUID `json:"event_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	PaymentIntentID uuid.UUID `json:"payment_intent_id"`
	MerchantID      uuid.UUID `json:"merchant_id"`
	AcquirerRef     string    `json:"acquirer_ref"`
	AmountCents     int64     `json:"amount_cents"`
	Currency        string    `json:"currency"`
}

// PaymentIntentCapturedV1 disparado no settlement.
type PaymentIntentCapturedV1 struct {
	EventID         uuid.UUID `json:"event_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	PaymentIntentID uuid.UUID `json:"payment_intent_id"`
	MerchantID      uuid.UUID `json:"merchant_id"`
	AmountCents     int64     `json:"amount_cents"`
	Currency        string    `json:"currency"`
}
