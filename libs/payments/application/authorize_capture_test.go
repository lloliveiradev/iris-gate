package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

// Extend fakeRepo with UpdateWithOutbox.
func (r *fakeRepo) UpdateWithOutbox(_ context.Context, p domain.PaymentIntent, ev application.OutboxMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.intents[p.ID] = p
	r.outbox = append(r.outbox, ev)
	return nil
}

type fakeAcquirer struct {
	mu       sync.Mutex
	approve  bool
	calls    int
	wantFail bool
}

func (a *fakeAcquirer) Authorize(_ context.Context, in application.AuthorizeInput) (application.AuthorizeOutput, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if a.wantFail {
		return application.AuthorizeOutput{}, errors.New("network error")
	}
	return application.AuthorizeOutput{
		AcquirerRef: "ref-" + in.IntentID.String(),
		Approved:    a.approve,
		Reason:      "test",
	}, nil
}

func seedIntent(t *testing.T, repo *fakeRepo, status domain.PaymentIntentStatus) domain.PaymentIntent {
	t.Helper()
	amt, _ := domain.NewMoney(12990, "BRL")
	pi, err := domain.NewPaymentIntent(uuid.New(), uuid.New(), amt, "tok_test_visa", "", time.Now())
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	pi.Status = status
	repo.mu.Lock()
	repo.intents[pi.ID] = pi
	repo.mu.Unlock()
	return pi
}

func TestAuthorize_Approved(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	acq := &fakeAcquirer{approve: true}
	pi := seedIntent(t, repo, domain.PaymentIntentPending)

	uc := application.NewAuthorizePaymentUseCase(repo, acq, fixedClock{t: time.Now()}, &seqIDs{})
	got, err := uc.Execute(context.Background(), pi.MerchantID, pi.ID)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if got.Status != domain.PaymentIntentAuthorized {
		t.Errorf("status = %v", got.Status)
	}
	if got.AuthorizedAt == nil {
		t.Error("AuthorizedAt nil")
	}
	if len(repo.outbox) != 1 {
		t.Errorf("outbox = %d, want 1", len(repo.outbox))
	}
	if topic := repo.outbox[0].Topic; topic != application.TopicPaymentIntentAuthorizedV1 {
		t.Errorf("topic = %q", topic)
	}
}

func TestAuthorize_Rejected(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	acq := &fakeAcquirer{approve: false}
	pi := seedIntent(t, repo, domain.PaymentIntentPending)

	uc := application.NewAuthorizePaymentUseCase(repo, acq, fixedClock{t: time.Now()}, &seqIDs{})
	_, err := uc.Execute(context.Background(), pi.MerchantID, pi.ID)
	if !errors.Is(err, domain.ErrAcquirerRejected) {
		t.Fatalf("err = %v, want ErrAcquirerRejected", err)
	}
}

func TestAuthorize_Idempotent(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	acq := &fakeAcquirer{approve: true}
	pi := seedIntent(t, repo, domain.PaymentIntentAuthorized)

	uc := application.NewAuthorizePaymentUseCase(repo, acq, fixedClock{t: time.Now()}, &seqIDs{})
	if _, err := uc.Execute(context.Background(), pi.MerchantID, pi.ID); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if acq.calls != 0 {
		t.Errorf("acquirer chamado %d vezes em retry — não é idempotente", acq.calls)
	}
}

func TestCapture_OK(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	pi := seedIntent(t, repo, domain.PaymentIntentAuthorized)

	uc := application.NewCapturePaymentUseCase(repo, fixedClock{t: time.Now()}, &seqIDs{})
	got, err := uc.Execute(context.Background(), pi.MerchantID, pi.ID)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if got.Status != domain.PaymentIntentCaptured {
		t.Errorf("status = %v", got.Status)
	}
	if len(repo.outbox) != 1 {
		t.Errorf("outbox = %d, want 1", len(repo.outbox))
	}
}

func TestCapture_FailsFromPending(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	pi := seedIntent(t, repo, domain.PaymentIntentPending)

	uc := application.NewCapturePaymentUseCase(repo, fixedClock{t: time.Now()}, &seqIDs{})
	_, err := uc.Execute(context.Background(), pi.MerchantID, pi.ID)
	if !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Fatalf("err = %v, want ErrInvalidStatusTransition", err)
	}
}

func TestCapture_Idempotent(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	pi := seedIntent(t, repo, domain.PaymentIntentCaptured)

	uc := application.NewCapturePaymentUseCase(repo, fixedClock{t: time.Now()}, &seqIDs{})
	if _, err := uc.Execute(context.Background(), pi.MerchantID, pi.ID); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(repo.outbox) != 0 {
		t.Errorf("outbox tem evento em retry — não é idempotente")
	}
}

func TestAuthorize_OtherMerchant_NotFoundAndUntouched(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	acq := &fakeAcquirer{approve: true}
	pi := seedIntent(t, repo, domain.PaymentIntentPending)

	uc := application.NewAuthorizePaymentUseCase(repo, acq, fixedClock{t: time.Now()}, &seqIDs{})
	_, err := uc.Execute(context.Background(), uuid.New(), pi.ID)
	if !errors.Is(err, domain.ErrPaymentIntentNotFound) {
		t.Fatalf("err = %v, want ErrPaymentIntentNotFound", err)
	}
	if acq.calls != 0 {
		t.Errorf("acquirer called %d times for a foreign intent", acq.calls)
	}
	if got := repo.intents[pi.ID]; got.Status != domain.PaymentIntentPending {
		t.Errorf("status = %v, want pending", got.Status)
	}
	if len(repo.outbox) != 0 {
		t.Errorf("outbox = %d, want 0", len(repo.outbox))
	}
}

func TestCapture_OtherMerchant_NotFoundAndUntouched(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	pi := seedIntent(t, repo, domain.PaymentIntentAuthorized)

	uc := application.NewCapturePaymentUseCase(repo, fixedClock{t: time.Now()}, &seqIDs{})
	_, err := uc.Execute(context.Background(), uuid.New(), pi.ID)
	if !errors.Is(err, domain.ErrPaymentIntentNotFound) {
		t.Fatalf("err = %v, want ErrPaymentIntentNotFound", err)
	}
	if got := repo.intents[pi.ID]; got.Status != domain.PaymentIntentAuthorized {
		t.Errorf("status = %v, want authorized", got.Status)
	}
	if len(repo.outbox) != 0 {
		t.Errorf("outbox = %d, want 0", len(repo.outbox))
	}
}
