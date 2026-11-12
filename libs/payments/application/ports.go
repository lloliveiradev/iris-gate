package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

// PaymentIntentRepo persists and reads PaymentIntents.
//
// Operations that change state must write the event to the outbox in the same
// transaction (Outbox pattern, ADR-0002).
type PaymentIntentRepo interface {
	CreateWithOutbox(ctx context.Context, p domain.PaymentIntent, ev OutboxMessage) error
	UpdateWithOutbox(ctx context.Context, p domain.PaymentIntent, ev OutboxMessage) error
	// GetByID loads an intent owned by merchantID. An intent that belongs to
	// another merchant is reported as domain.ErrPaymentIntentNotFound, exactly
	// like an id that does not exist, so callers cannot probe other tenants.
	GetByID(ctx context.Context, merchantID, id uuid.UUID) (domain.PaymentIntent, error)
	ListByMerchant(ctx context.Context, merchantID uuid.UUID, limit int) ([]domain.PaymentIntent, error)
}

// OutboxMessage describes a message the relay worker publishes.
type OutboxMessage struct {
	Topic   string
	Key     string
	Payload []byte
	Headers map[string]string
}

// Acquirer is the acquirer port (Stone, Cielo, etc.).
// The fake acquirer always approves.
type Acquirer interface {
	Authorize(ctx context.Context, in AuthorizeInput) (AuthorizeOutput, error)
}

type AuthorizeInput struct {
	IntentID    uuid.UUID
	AmountCents int64
	Currency    string
	CardToken   string
}

type AuthorizeOutput struct {
	AcquirerRef string
	Approved    bool
	Reason      string
}

type Clock interface {
	Now() time.Time
}

type IDGen interface {
	NewUUID() uuid.UUID
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

type UUIDGen struct{}

func (UUIDGen) NewUUID() uuid.UUID { return uuid.New() }
