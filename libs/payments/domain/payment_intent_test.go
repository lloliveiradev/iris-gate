package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

func validIntent(t *testing.T) (domain.PaymentIntent, time.Time) {
	t.Helper()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	amt, _ := domain.NewMoney(12990, "BRL")
	pi, err := domain.NewPaymentIntent(uuid.New(), uuid.New(), amt, "tok_abc12345", "ACME order", now)
	if err != nil {
		t.Fatalf("NewPaymentIntent: %v", err)
	}
	return pi, now
}

func TestNewPaymentIntent_OK(t *testing.T) {
	t.Parallel()
	pi, now := validIntent(t)
	if pi.Status != domain.PaymentIntentPending {
		t.Errorf("status = %v, want pending", pi.Status)
	}
	if !pi.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v", pi.CreatedAt)
	}
}

func TestNewPaymentIntent_Invalid(t *testing.T) {
	t.Parallel()
	amt, _ := domain.NewMoney(12990, "BRL")
	now := time.Now()
	mid := uuid.New()
	cases := map[string]func() (uuid.UUID, uuid.UUID, domain.Money, string, string){
		"nil merchant":    func() (uuid.UUID, uuid.UUID, domain.Money, string, string) { return uuid.New(), uuid.Nil, amt, "tok_abc12345", "" },
		"empty money":     func() (uuid.UUID, uuid.UUID, domain.Money, string, string) { return uuid.New(), mid, domain.Money{}, "tok_abc12345", "" },
		"short token":     func() (uuid.UUID, uuid.UUID, domain.Money, string, string) { return uuid.New(), mid, amt, "tok_a", "" },
		"long token":      func() (uuid.UUID, uuid.UUID, domain.Money, string, string) { return uuid.New(), mid, amt, strings.Repeat("x", 257), "" },
		"long description": func() (uuid.UUID, uuid.UUID, domain.Money, string, string) { return uuid.New(), mid, amt, "tok_abc12345", strings.Repeat("d", 201) },
	}
	for name, factory := range cases {
		factory := factory
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			id, m, money, token, desc := factory()
			_, err := domain.NewPaymentIntent(id, m, money, token, desc, now)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("err = %v, want ErrValidation", err)
			}
		})
	}
}
