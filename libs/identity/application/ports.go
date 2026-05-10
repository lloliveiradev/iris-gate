package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

// Outbound ports.

type MerchantRepo interface {
	Create(ctx context.Context, m domain.Merchant) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Merchant, error)
	GetByDocument(ctx context.Context, doc string) (domain.Merchant, error)
}

type APIKeyRepo interface {
	Create(ctx context.Context, k domain.APIKey) error
	GetByPrefix(ctx context.Context, prefix string) (domain.APIKey, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.APIKey, error)
	Revoke(ctx context.Context, id uuid.UUID, at time.Time) (domain.APIKey, error)
	TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error
}

type Hasher interface {
	Hash(secret string) (string, error)
	Verify(secret, hash string) (bool, error)
}

// SecretGen produz prefixo público + segredo random pra um token novo.
type SecretGen interface {
	NewToken() (prefix, secret string, err error)
}

type Clock interface {
	Now() time.Time
}

type IDGen interface {
	NewUUID() uuid.UUID
}

// Default implementations: composables baseadas na stdlib + google/uuid.

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

type UUIDGen struct{}

func (UUIDGen) NewUUID() uuid.UUID { return uuid.New() }
