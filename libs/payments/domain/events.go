package domain

import (
	"time"

	"github.com/google/uuid"
)

// PaymentIntentCreatedV1 is the payload of `payments.payment_intent.created.v1`,
// published through the outbox (ADR-0002) when a PaymentIntent is created.
type PaymentIntentCreatedV1 struct {
	EventID         uuid.UUID `json:"event_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	PaymentIntentID uuid.UUID `json:"payment_intent_id"`
	MerchantID      uuid.UUID `json:"merchant_id"`
	AmountCents     int64     `json:"amount_cents"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
}

// PaymentIntentAuthorizedV1 is emitted when the acquirer approves.
type PaymentIntentAuthorizedV1 struct {
	EventID         uuid.UUID `json:"event_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	PaymentIntentID uuid.UUID `json:"payment_intent_id"`
	MerchantID      uuid.UUID `json:"merchant_id"`
	AcquirerRef     string    `json:"acquirer_ref"`
	AmountCents     int64     `json:"amount_cents"`
	Currency        string    `json:"currency"`
}

// PaymentIntentCapturedV1 is emitted on capture.
type PaymentIntentCapturedV1 struct {
	EventID         uuid.UUID `json:"event_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	PaymentIntentID uuid.UUID `json:"payment_intent_id"`
	MerchantID      uuid.UUID `json:"merchant_id"`
	AmountCents     int64     `json:"amount_cents"`
	Currency        string    `json:"currency"`
}

// PaymentIntentFailedV1 is emitted when the acquirer declines the
// authorization and the intent is persisted as failed.
type PaymentIntentFailedV1 struct {
	EventID         uuid.UUID `json:"event_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	PaymentIntentID uuid.UUID `json:"payment_intent_id"`
	MerchantID      uuid.UUID `json:"merchant_id"`
	FailureCode     string    `json:"failure_code"`
	AmountCents     int64     `json:"amount_cents"`
	Currency        string    `json:"currency"`
}
