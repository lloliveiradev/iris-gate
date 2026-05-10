package application

import (
	"context"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

type IssueAPIKeyInput struct {
	MerchantID uuid.UUID
	Scopes     []string
}

// IssueAPIKeyOutput devolve a API key persistida + o segredo em texto-claro.
// O segredo só é visível **uma vez** — após este retorno, apenas o hash sobrevive.
type IssueAPIKeyOutput struct {
	APIKey domain.APIKey
	Token  string // formato `irk_<prefix>.<secret>`
}

type IssueAPIKeyUseCase struct {
	merchants MerchantRepo
	keys      APIKeyRepo
	hasher    Hasher
	secrets   SecretGen
	clock     Clock
	ids       IDGen
}

func NewIssueAPIKeyUseCase(
	merchants MerchantRepo,
	keys APIKeyRepo,
	hasher Hasher,
	secrets SecretGen,
	clock Clock,
	ids IDGen,
) *IssueAPIKeyUseCase {
	return &IssueAPIKeyUseCase{
		merchants: merchants,
		keys:      keys,
		hasher:    hasher,
		secrets:   secrets,
		clock:     clock,
		ids:       ids,
	}
}

func (uc *IssueAPIKeyUseCase) Execute(ctx context.Context, in IssueAPIKeyInput) (IssueAPIKeyOutput, error) {
	if _, err := uc.merchants.GetByID(ctx, in.MerchantID); err != nil {
		return IssueAPIKeyOutput{}, err
	}

	prefix, secret, err := uc.secrets.NewToken()
	if err != nil {
		return IssueAPIKeyOutput{}, err
	}
	hash, err := uc.hasher.Hash(secret)
	if err != nil {
		return IssueAPIKeyOutput{}, err
	}

	scopes := in.Scopes
	if scopes == nil {
		scopes = []string{}
	}

	now := uc.clock.Now()
	k := domain.APIKey{
		ID:         uc.ids.NewUUID(),
		MerchantID: in.MerchantID,
		Prefix:     prefix,
		Hash:       hash,
		Status:     domain.APIKeyActive,
		Scopes:     scopes,
		CreatedAt:  now,
	}
	if err := uc.keys.Create(ctx, k); err != nil {
		return IssueAPIKeyOutput{}, err
	}

	return IssueAPIKeyOutput{
		APIKey: k,
		Token:  domain.FormatToken(prefix, secret),
	}, nil
}
