package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

type CreateMerchantInput struct {
	LegalName    string
	Document     string
	ContactEmail string
}

type CreateMerchantUseCase struct {
	repo  MerchantRepo
	clock Clock
	ids   IDGen
}

func NewCreateMerchantUseCase(repo MerchantRepo, clock Clock, ids IDGen) *CreateMerchantUseCase {
	return &CreateMerchantUseCase{repo: repo, clock: clock, ids: ids}
}

func (uc *CreateMerchantUseCase) Execute(ctx context.Context, in CreateMerchantInput) (domain.Merchant, error) {
	m, err := domain.NewMerchant(uc.ids.NewUUID(), in.LegalName, in.Document, in.ContactEmail, uc.clock.Now())
	if err != nil {
		return domain.Merchant{}, err
	}

	if existing, err := uc.repo.GetByDocument(ctx, m.Document); err == nil {
		_ = existing
		return domain.Merchant{}, fmt.Errorf("%w", domain.ErrDocumentExists)
	} else if !errors.Is(err, domain.ErrMerchantNotFound) {
		return domain.Merchant{}, err
	}

	if err := uc.repo.Create(ctx, m); err != nil {
		return domain.Merchant{}, err
	}
	return m, nil
}
