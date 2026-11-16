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
	// UpdateWithOutbox saves p only while the stored row still has p.Version
	// for p.ID and p.MerchantID, increments the version, and writes ev in the
	// same transaction. A version mismatch writes nothing and returns
	// domain.ErrConcurrentUpdate; an intent missing for that merchant returns
	// domain.ErrPaymentIntentNotFound.
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
//
// Authorize is idempotent per AuthorizeInput.IdempotencyKey: a repeated key
// returns the result of the first call and authorizes nothing new.
type Acquirer interface {
	Authorize(ctx context.Context, in AuthorizeInput) (AuthorizeOutput, error)
}

// AuthorizeInput is one authorization request to the acquirer.
type AuthorizeInput struct {
	IntentID    uuid.UUID
	AmountCents int64
	Currency    string
	CardToken   string
	// IdempotencyKey is required. The authorize use case passes the intent id,
	// so concurrent or retried authorizations of one intent reach the acquirer
	// as one operation.
	IdempotencyKey string
}

// AuthorizeOutput is the acquirer's decision. Approved=false is a decline,
// not an error; Reason is free text from the acquirer.
type AuthorizeOutput struct {
	AcquirerRef string
	Approved    bool
	Reason      string
}

// Clock is the time source port.
type Clock interface {
	Now() time.Time
}

// IDGen generates ids for aggregates and events.
type IDGen interface {
	NewUUID() uuid.UUID
}

// SystemClock is the real Clock, in UTC.
type SystemClock struct{}

// Now returns the current UTC time.
func (SystemClock) Now() time.Time { return time.Now().UTC() }

// UUIDGen is the real IDGen, with random v4 UUIDs.
type UUIDGen struct{}

// NewUUID returns a random UUID.
func (UUIDGen) NewUUID() uuid.UUID { return uuid.New() }
