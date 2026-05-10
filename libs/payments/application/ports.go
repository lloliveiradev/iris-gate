package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

// PaymentIntentRepo persiste e lê PaymentIntents.
//
// Operações que mudam estado **devem** escrever o evento na outbox na mesma
// transação — invariante crítica do padrão Outbox (ADR-0002).
type PaymentIntentRepo interface {
	CreateWithOutbox(ctx context.Context, p domain.PaymentIntent, ev OutboxMessage) error
	UpdateWithOutbox(ctx context.Context, p domain.PaymentIntent, ev OutboxMessage) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.PaymentIntent, error)
	GetByMerchantAndIdempotencyKey(ctx context.Context, merchantID uuid.UUID, key string) (domain.PaymentIntent, error)
}

// OutboxMessage descreve uma mensagem a ser publicada via relay worker.
type OutboxMessage struct {
	Topic   string
	Key     string
	Payload []byte
	Headers map[string]string
}

// Acquirer é o port de adquirente (Stone, Cielo, etc.).
// FakeAcquirer no MVP devolve sempre Approved.
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
