package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/iris-gate/iris-gate/libs/identity/application"
	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

type harness struct {
	merchants *fakeMerchantRepo
	keys      *fakeAPIKeyRepo
	hasher    fakeHasher
	secrets   *seqSecrets
	clock     fixedClock
	ids       *seqIDs
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return &harness{
		merchants: newFakeMerchantRepo(),
		keys:      newFakeAPIKeyRepo(),
		hasher:    fakeHasher{},
		secrets:   &seqSecrets{},
		clock:     fixedClock{t: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)},
		ids:       &seqIDs{},
	}
}

func TestCreateMerchant_OK(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	uc := application.NewCreateMerchantUseCase(h.merchants, h.clock, h.ids)
	m, err := uc.Execute(context.Background(), application.CreateMerchantInput{
		LegalName:    "ACME LTDA",
		Document:     "12345678000199",
		ContactEmail: "ops@acme.test",
	})
	if err != nil {
		t.Fatalf("Execute err = %v", err)
	}
	if m.Status != domain.MerchantPending {
		t.Errorf("status = %v", m.Status)
	}

	got, err := h.merchants.GetByID(context.Background(), m.ID)
	if err != nil || got.ID != m.ID {
		t.Fatalf("merchant não persistido: err=%v got=%v", err, got)
	}
}

func TestCreateMerchant_DuplicateDocument(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	uc := application.NewCreateMerchantUseCase(h.merchants, h.clock, h.ids)

	in := application.CreateMerchantInput{LegalName: "ACME", Document: "12345678000199", ContactEmail: "a@b.com"}
	if _, err := uc.Execute(context.Background(), in); err != nil {
		t.Fatalf("first execute: %v", err)
	}
	_, err := uc.Execute(context.Background(), in)
	if !errors.Is(err, domain.ErrDocumentExists) {
		t.Fatalf("err = %v, want ErrDocumentExists", err)
	}
}

func TestIssueAPIKey_OK(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	createUC := application.NewCreateMerchantUseCase(h.merchants, h.clock, h.ids)
	m, err := createUC.Execute(context.Background(), application.CreateMerchantInput{
		LegalName: "ACME", Document: "12345678000199", ContactEmail: "a@b.com",
	})
	if err != nil {
		t.Fatalf("create merchant: %v", err)
	}

	issueUC := application.NewIssueAPIKeyUseCase(h.merchants, h.keys, h.hasher, h.secrets, h.clock, h.ids)
	out, err := issueUC.Execute(context.Background(), application.IssueAPIKeyInput{MerchantID: m.ID})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if out.Token == "" {
		t.Fatal("token vazio")
	}
	if out.APIKey.Hash == "" || out.APIKey.Hash == "secret1" {
		t.Errorf("hash deveria ser derivado, got %q", out.APIKey.Hash)
	}
}

func TestAuthenticateAPIKey_FullFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	createUC := application.NewCreateMerchantUseCase(h.merchants, h.clock, h.ids)
	m, _ := createUC.Execute(context.Background(), application.CreateMerchantInput{
		LegalName: "ACME", Document: "12345678000199", ContactEmail: "a@b.com",
	})

	issueUC := application.NewIssueAPIKeyUseCase(h.merchants, h.keys, h.hasher, h.secrets, h.clock, h.ids)
	out, _ := issueUC.Execute(context.Background(), application.IssueAPIKeyInput{MerchantID: m.ID})

	authUC := application.NewAuthenticateAPIKeyUseCase(h.keys, h.merchants, h.hasher, h.clock)

	t.Run("token válido", func(t *testing.T) {
		got, err := authUC.Execute(context.Background(), out.Token)
		if err != nil {
			t.Fatalf("auth válido falhou: %v", err)
		}
		if got.Merchant.ID != m.ID {
			t.Fatalf("merchant errado")
		}
	})

	t.Run("token malformado", func(t *testing.T) {
		_, err := authUC.Execute(context.Background(), "bogus")
		if !errors.Is(err, domain.ErrAPIKeyInvalid) {
			t.Fatalf("err = %v, want ErrAPIKeyInvalid", err)
		}
	})

	t.Run("secret errado", func(t *testing.T) {
		_, err := authUC.Execute(context.Background(), domain.FormatToken(out.APIKey.Prefix, "wrong"))
		if !errors.Is(err, domain.ErrAPIKeyInvalid) {
			t.Fatalf("err = %v, want ErrAPIKeyInvalid", err)
		}
	})

	t.Run("token revogado", func(t *testing.T) {
		revokeUC := application.NewRevokeAPIKeyUseCase(h.keys, h.clock)
		if _, err := revokeUC.Execute(context.Background(), out.APIKey.ID); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		_, err := authUC.Execute(context.Background(), out.Token)
		if !errors.Is(err, domain.ErrAPIKeyRevoked) {
			t.Fatalf("err = %v, want ErrAPIKeyRevoked", err)
		}
	})
}
