package domain_test

import (
	"errors"
	"testing"

	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

func TestNewMoney_OK(t *testing.T) {
	t.Parallel()
	m, err := domain.NewMoney(12990, "BRL")
	if err != nil {
		t.Fatalf("NewMoney err = %v", err)
	}
	if m.AmountCents != 12990 || m.Currency != "BRL" {
		t.Errorf("got %+v", m)
	}
	if got := m.String(); got != "12990 BRL" {
		t.Errorf("String() = %q", got)
	}
}

func TestNewMoney_Invalid(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		amount   int64
		currency string
	}{
		"zero amount":     {0, "BRL"},
		"negative amount": {-1, "BRL"},
		"lowercase":       {100, "brl"},
		"too short":       {100, "BR"},
		"too long":        {100, "BRLE"},
		"with digit":      {100, "BR1"},
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewMoney(tc.amount, tc.currency)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("err = %v, want ErrValidation", err)
			}
		})
	}
}

func TestMoney_Equals(t *testing.T) {
	t.Parallel()
	a, _ := domain.NewMoney(100, "BRL")
	b, _ := domain.NewMoney(100, "BRL")
	c, _ := domain.NewMoney(100, "USD")
	if !a.Equals(b) {
		t.Errorf("equal moneys not equal")
	}
	if a.Equals(c) {
		t.Errorf("different currency considered equal")
	}
}
