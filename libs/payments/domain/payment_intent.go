package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PaymentIntentStatus string

const (
	PaymentIntentPending    PaymentIntentStatus = "pending"
	PaymentIntentAuthorized PaymentIntentStatus = "authorized"
	PaymentIntentCaptured   PaymentIntentStatus = "captured"
	PaymentIntentRefunded   PaymentIntentStatus = "refunded"
	PaymentIntentCanceled   PaymentIntentStatus = "canceled"
	PaymentIntentFailed     PaymentIntentStatus = "failed"
)

type PaymentIntent struct {
	ID           uuid.UUID
	MerchantID   uuid.UUID
	Amount       Money
	Status       PaymentIntentStatus
	CardToken    string
	Description  string
	CreatedAt    time.Time
	AuthorizedAt *time.Time
	CapturedAt   *time.Time
}

// NewPaymentIntent valida e cria um intent no estado inicial `pending`.
//
// id e now são injetados — sem dependência de I/O ou clock real no domínio.
func NewPaymentIntent(id, merchantID uuid.UUID, amount Money, cardToken, description string, now time.Time) (PaymentIntent, error) {
	if merchantID == uuid.Nil {
		return PaymentIntent{}, fmt.Errorf("%w: merchant_id required", ErrValidation)
	}
	if amount.IsZero() {
		return PaymentIntent{}, fmt.Errorf("%w: amount required", ErrValidation)
	}
	if l := len(cardToken); l < 8 || l > 256 {
		return PaymentIntent{}, fmt.Errorf("%w: card_token length out of range (%d)", ErrValidation, l)
	}
	if len(description) > 200 {
		return PaymentIntent{}, fmt.Errorf("%w: description > 200 chars", ErrValidation)
	}
	return PaymentIntent{
		ID:          id,
		MerchantID:  merchantID,
		Amount:      amount,
		Status:      PaymentIntentPending,
		CardToken:   cardToken,
		Description: description,
		CreatedAt:   now,
	}, nil
}
