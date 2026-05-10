package application

import (
	"context"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

type RevokeAPIKeyUseCase struct {
	keys  APIKeyRepo
	clock Clock
}

func NewRevokeAPIKeyUseCase(keys APIKeyRepo, clock Clock) *RevokeAPIKeyUseCase {
	return &RevokeAPIKeyUseCase{keys: keys, clock: clock}
}

func (uc *RevokeAPIKeyUseCase) Execute(ctx context.Context, id uuid.UUID) (domain.APIKey, error) {
	return uc.keys.Revoke(ctx, id, uc.clock.Now())
}
