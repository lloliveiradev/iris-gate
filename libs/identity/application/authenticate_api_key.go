package application

import (
	"context"
	"fmt"

	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

type AuthenticateAPIKeyOutput struct {
	APIKey   domain.APIKey
	Merchant domain.Merchant
}

type AuthenticateAPIKeyUseCase struct {
	keys      APIKeyRepo
	merchants MerchantRepo
	hasher    Hasher
	clock     Clock
}

func NewAuthenticateAPIKeyUseCase(keys APIKeyRepo, merchants MerchantRepo, hasher Hasher, clock Clock) *AuthenticateAPIKeyUseCase {
	return &AuthenticateAPIKeyUseCase{keys: keys, merchants: merchants, hasher: hasher, clock: clock}
}

// Execute valida um token e devolve a API key + merchant dono.
//   - token malformado / prefix inexistente / hash não bate → ErrAPIKeyInvalid (401)
//   - chave revogada → ErrAPIKeyRevoked (403)
func (uc *AuthenticateAPIKeyUseCase) Execute(ctx context.Context, token string) (AuthenticateAPIKeyOutput, error) {
	prefix, secret, err := domain.ParseToken(token)
	if err != nil {
		return AuthenticateAPIKeyOutput{}, err
	}

	k, err := uc.keys.GetByPrefix(ctx, prefix)
	if err != nil {
		// Repo deve devolver ErrAPIKeyNotFound; mapeamos pra Invalid pra evitar
		// vazar diferença entre "prefix existe mas secret errou" e "prefix não existe".
		return AuthenticateAPIKeyOutput{}, fmt.Errorf("%w: lookup", domain.ErrAPIKeyInvalid)
	}

	ok, err := uc.hasher.Verify(secret, k.Hash)
	if err != nil {
		return AuthenticateAPIKeyOutput{}, err
	}
	if !ok {
		return AuthenticateAPIKeyOutput{}, fmt.Errorf("%w: hash mismatch", domain.ErrAPIKeyInvalid)
	}

	if k.IsRevoked() {
		return AuthenticateAPIKeyOutput{}, domain.ErrAPIKeyRevoked
	}

	m, err := uc.merchants.GetByID(ctx, k.MerchantID)
	if err != nil {
		return AuthenticateAPIKeyOutput{}, err
	}

	// Best-effort: marca uso (não bloqueia auth se falhar).
	_ = uc.keys.TouchLastUsed(ctx, k.ID, uc.clock.Now())

	return AuthenticateAPIKeyOutput{APIKey: k, Merchant: m}, nil
}
