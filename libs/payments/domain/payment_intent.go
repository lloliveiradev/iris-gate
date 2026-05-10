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

// MarkAuthorized transiciona pending → authorized.
// Idempotente quando já está authorized — não retorna erro pra evitar
// que o caller precise distinguir "primeira chamada" vs "retry".
//
// Outras transições (captured, canceled, failed) → ErrInvalidStatusTransition.
func (p *PaymentIntent) MarkAuthorized(at time.Time) error {
	switch p.Status {
	case PaymentIntentPending, PaymentIntentAuthorized:
		p.Status = PaymentIntentAuthorized
		p.AuthorizedAt = &at
		return nil
	default:
		return fmt.Errorf("%w: cannot authorize from %s", ErrInvalidStatusTransition, p.Status)
	}
}

// MarkCaptured transiciona authorized → captured.
// Idempotente quando já está captured.
func (p *PaymentIntent) MarkCaptured(at time.Time) error {
	switch p.Status {
	case PaymentIntentAuthorized, PaymentIntentCaptured:
		p.Status = PaymentIntentCaptured
		p.CapturedAt = &at
		return nil
	default:
		return fmt.Errorf("%w: cannot capture from %s", ErrInvalidStatusTransition, p.Status)
	}
}

// MarkFailed registra rejeição do adquirente.
func (p *PaymentIntent) MarkFailed() error {
	if p.Status != PaymentIntentPending {
		return fmt.Errorf("%w: cannot fail from %s", ErrInvalidStatusTransition, p.Status)
	}
	p.Status = PaymentIntentFailed
	return nil
}
