package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/domain"
	"github.com/iris-gate/iris-gate/libs/payments/testfixtures"
)

func validIntent(t *testing.T) (domain.PaymentIntent, time.Time) {
	t.Helper()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	amt, _ := domain.NewMoney(12990, "BRL")
	pi, err := domain.NewPaymentIntent(uuid.New(), uuid.New(), amt, testfixtures.TokenApproved, "ACME order", now)
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
	if pi.Version != 0 || pi.FailureCode != "" {
		t.Errorf("version = %d failure_code = %q, want 0 and empty", pi.Version, pi.FailureCode)
	}
}

func TestNewPaymentIntent_Invalid(t *testing.T) {
	t.Parallel()
	amt, _ := domain.NewMoney(12990, "BRL")
	now := time.Now()
	mid := uuid.New()
	cases := map[string]func() (uuid.UUID, uuid.UUID, domain.Money, string, string){
		"nil merchant": func() (uuid.UUID, uuid.UUID, domain.Money, string, string) {
			return uuid.New(), uuid.Nil, amt, testfixtures.TokenApproved, ""
		},
		"empty money": func() (uuid.UUID, uuid.UUID, domain.Money, string, string) {
			return uuid.New(), mid, domain.Money{}, testfixtures.TokenApproved, ""
		},
		"short token": func() (uuid.UUID, uuid.UUID, domain.Money, string, string) {
			return uuid.New(), mid, amt, testfixtures.TokenTooShort, ""
		},
		"long token": func() (uuid.UUID, uuid.UUID, domain.Money, string, string) {
			return uuid.New(), mid, amt, strings.Repeat("x", 257), ""
		},
		"long description": func() (uuid.UUID, uuid.UUID, domain.Money, string, string) {
			return uuid.New(), mid, amt, testfixtures.TokenApproved, strings.Repeat("d", 201)
		},
	}
	for name, factory := range cases {
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

// P7: MarkFailed records the code and returns one failed event; twice is a no-op.
func TestMarkFailed_RecordsCodeAndReturnsEventOnce(t *testing.T) {
	t.Parallel()
	pi, now := validIntent(t)
	eventID := uuid.New()

	ev, err := pi.MarkFailed(domain.FailureCardDeclined, eventID, now)
	if err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if pi.Status != domain.PaymentIntentFailed || pi.FailureCode != domain.FailureCardDeclined {
		t.Errorf("status = %v code = %q, want failed card_declined", pi.Status, pi.FailureCode)
	}
	if ev == nil {
		t.Fatal("event = nil, want one failed event")
	}
	want := domain.PaymentIntentFailedV1{
		EventID:         eventID,
		OccurredAt:      now,
		PaymentIntentID: pi.ID,
		MerchantID:      pi.MerchantID,
		FailureCode:     string(domain.FailureCardDeclined),
		AmountCents:     pi.Amount.AmountCents,
		Currency:        pi.Amount.Currency,
	}
	if *ev != want {
		t.Errorf("event = %+v, want %+v", *ev, want)
	}

	again, err := pi.MarkFailed(domain.FailureCardDeclined, uuid.New(), now)
	if err != nil || again != nil {
		t.Errorf("second MarkFailed = (%v, %v), want (nil, nil)", again, err)
	}
}

func TestMarkFailed_RejectsOtherStatusesAndUnknownCodes(t *testing.T) {
	t.Parallel()
	for _, status := range []domain.PaymentIntentStatus{domain.PaymentIntentAuthorized, domain.PaymentIntentCaptured} {
		pi, now := validIntent(t)
		pi.Status = status
		if _, err := pi.MarkFailed(domain.FailureCardDeclined, uuid.New(), now); !errors.Is(err, domain.ErrInvalidStatusTransition) {
			t.Errorf("from %s: err = %v, want ErrInvalidStatusTransition", status, err)
		}
		if pi.Status != status || pi.FailureCode != "" {
			t.Errorf("from %s: intent changed to %s/%q", status, pi.Status, pi.FailureCode)
		}
	}

	pi, now := validIntent(t)
	if _, err := pi.MarkFailed("bogus", uuid.New(), now); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("unknown code: err = %v, want ErrValidation", err)
	}
	if pi.Status != domain.PaymentIntentPending {
		t.Errorf("unknown code changed status to %s", pi.Status)
	}
}

func TestAuthorizationNeeded(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status  domain.PaymentIntentStatus
		needed  bool
		wantErr error
	}{
		{domain.PaymentIntentPending, true, nil},
		{domain.PaymentIntentAuthorized, false, nil},
		{domain.PaymentIntentCaptured, false, domain.ErrInvalidStatusTransition},
		{domain.PaymentIntentFailed, false, domain.ErrInvalidStatusTransition},
	}
	for _, c := range cases {
		pi, _ := validIntent(t)
		pi.Status = c.status
		needed, err := pi.AuthorizationNeeded()
		if needed != c.needed || !errors.Is(err, c.wantErr) {
			t.Errorf("%s: (%v, %v), want (%v, %v)", c.status, needed, err, c.needed, c.wantErr)
		}
	}
}
