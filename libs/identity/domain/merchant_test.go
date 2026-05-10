package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

func TestNewMerchant_OK(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	m, err := domain.NewMerchant(id, "ACME Pagamentos LTDA", "12345678000199", "ops@acme.test", now)
	if err != nil {
		t.Fatalf("NewMerchant err = %v", err)
	}
	if m.ID != id {
		t.Errorf("ID = %v, want %v", m.ID, id)
	}
	if m.Status != domain.MerchantPending {
		t.Errorf("Status = %v, want pending", m.Status)
	}
	if !m.CreatedAt.Equal(now) || !m.UpdatedAt.Equal(now) {
		t.Errorf("timestamps = %v / %v, want %v", m.CreatedAt, m.UpdatedAt, now)
	}
}

func TestNewMerchant_Invalid(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name, doc, email string
	}{
		"empty name":   {"", "12345678000199", "a@b.com"},
		"name too big": {strings.Repeat("x", 201), "12345678000199", "a@b.com"},
		"bad doc":      {"ok", "abc", "a@b.com"},
		"short doc":    {"ok", "1234567890", "a@b.com"},
		"bad email":    {"ok", "12345678000199", "not-an-email"},
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewMerchant(uuid.New(), tc.name, tc.doc, tc.email, time.Now())
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("err = %v, want ErrValidation", err)
			}
		})
	}
}
