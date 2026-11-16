package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/domain"
	"github.com/iris-gate/iris-gate/libs/payments/testfixtures"
)

// UpdateWithOutbox mirrors the Postgres contract: it saves only while the
// stored version matches, increments it, and appends the outbox message.
func (r *fakeRepo) UpdateWithOutbox(_ context.Context, p domain.PaymentIntent, ev application.OutboxMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.intents[p.ID]
	if !ok || stored.MerchantID != p.MerchantID {
		return domain.ErrPaymentIntentNotFound
	}
	if stored.Version != p.Version {
		return domain.ErrConcurrentUpdate
	}
	p.Version++
	r.intents[p.ID] = p
	r.outbox = append(r.outbox, ev)
	return nil
}

// failingUpdateRepo fails the next `failures` updates with err, writing nothing.
type failingUpdateRepo struct {
	*fakeRepo
	err      error
	failures int
}

func (r *failingUpdateRepo) UpdateWithOutbox(ctx context.Context, p domain.PaymentIntent, ev application.OutboxMessage) error {
	if r.failures > 0 {
		r.failures--
		return r.err
	}
	return r.fakeRepo.UpdateWithOutbox(ctx, p, ev)
}

// fakeAcquirer answers every call with the same decision and records the
// idempotency keys it received.
type fakeAcquirer struct {
	mu       sync.Mutex
	approve  bool
	calls    int
	keys     []string
	wantFail bool
}

func (a *fakeAcquirer) Authorize(_ context.Context, in application.AuthorizeInput) (application.AuthorizeOutput, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.keys = append(a.keys, in.IdempotencyKey)
	if a.wantFail {
		return application.AuthorizeOutput{}, errors.New("network error")
	}
	return application.AuthorizeOutput{
		AcquirerRef: "ref-" + in.IdempotencyKey,
		Approved:    a.approve,
		Reason:      "test",
	}, nil
}

func seedIntent(t *testing.T, repo *fakeRepo, status domain.PaymentIntentStatus) domain.PaymentIntent {
	t.Helper()
	amt, _ := domain.NewMoney(12990, "BRL")
	pi, err := domain.NewPaymentIntent(uuid.New(), uuid.New(), amt, testfixtures.TokenApproved, "", time.Now())
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	pi.Status = status
	repo.mu.Lock()
	repo.intents[pi.ID] = pi
	repo.mu.Unlock()
	return pi
}

func newAuthorize(repo application.PaymentIntentRepo, acq application.Acquirer) *application.AuthorizePaymentUseCase {
	return application.NewAuthorizePaymentUseCase(repo, acq, fixedClock{t: time.Now()}, &seqIDs{})
}

func TestAuthorize_Approved(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	acq := &fakeAcquirer{approve: true}
	pi := seedIntent(t, repo, domain.PaymentIntentPending)

	got, err := newAuthorize(repo, acq).Execute(context.Background(), pi.MerchantID, pi.ID)
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
		t.Fatalf("outbox = %d, want 1", len(repo.outbox))
	}
	if topic := repo.outbox[0].Topic; topic != application.TopicPaymentIntentAuthorizedV1 {
		t.Errorf("topic = %q", topic)
	}
	if len(acq.keys) != 1 || acq.keys[0] != pi.ID.String() {
		t.Errorf("acquirer idempotency keys = %v, want [%s]", acq.keys, pi.ID)
	}
}

// D3: a decline is persisted as failed with a failure code and a failed event.
func TestAuthorize_Rejected_PersistsFailedWithEvent(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	acq := &fakeAcquirer{approve: false}
	pi := seedIntent(t, repo, domain.PaymentIntentPending)

	got, err := newAuthorize(repo, acq).Execute(context.Background(), pi.MerchantID, pi.ID)
	if !errors.Is(err, domain.ErrAcquirerRejected) {
		t.Fatalf("err = %v, want ErrAcquirerRejected", err)
	}
	if got.Status != domain.PaymentIntentFailed || got.FailureCode != domain.FailureCardDeclined {
		t.Errorf("returned intent = %s/%q, want failed/card_declined", got.Status, got.FailureCode)
	}
	stored := repo.intents[pi.ID]
	if stored.Status != domain.PaymentIntentFailed || stored.FailureCode != domain.FailureCardDeclined {
		t.Errorf("stored intent = %s/%q, want failed/card_declined", stored.Status, stored.FailureCode)
	}
	if len(repo.outbox) != 1 || repo.outbox[0].Topic != application.TopicPaymentIntentFailedV1 {
		t.Fatalf("outbox = %+v, want one %s message", repo.outbox, application.TopicPaymentIntentFailedV1)
	}
	var ev domain.PaymentIntentFailedV1
	if err := json.Unmarshal(repo.outbox[0].Payload, &ev); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if ev.PaymentIntentID != pi.ID || ev.FailureCode != string(domain.FailureCardDeclined) {
		t.Errorf("event = %+v", ev)
	}
}

// P1: authorize twice returns the authorized intent, one acquirer call, one event.
func TestAuthorize_Idempotent(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	acq := &fakeAcquirer{approve: true}
	pi := seedIntent(t, repo, domain.PaymentIntentPending)
	uc := newAuthorize(repo, acq)

	for range 2 {
		got, err := uc.Execute(context.Background(), pi.MerchantID, pi.ID)
		if err != nil || got.Status != domain.PaymentIntentAuthorized {
			t.Fatalf("authorize = (%v, %v)", got.Status, err)
		}
	}
	if acq.calls != 1 {
		t.Errorf("acquirer called %d times on retry, want 1", acq.calls)
	}
	if len(repo.outbox) != 1 {
		t.Errorf("outbox = %d, want 1", len(repo.outbox))
	}
}

// P4, P5: captured or failed intents never reach the acquirer.
func TestAuthorize_NonPending_DoesNotCallAcquirer(t *testing.T) {
	t.Parallel()
	for _, status := range []domain.PaymentIntentStatus{domain.PaymentIntentCaptured, domain.PaymentIntentFailed} {
		repo := newFakeRepo()
		acq := &fakeAcquirer{approve: true}
		pi := seedIntent(t, repo, status)

		_, err := newAuthorize(repo, acq).Execute(context.Background(), pi.MerchantID, pi.ID)
		if !errors.Is(err, domain.ErrInvalidStatusTransition) {
			t.Errorf("%s: err = %v, want ErrInvalidStatusTransition", status, err)
		}
		if acq.calls != 0 || len(repo.outbox) != 0 {
			t.Errorf("%s: acquirer calls = %d outbox = %d, want 0 and 0", status, acq.calls, len(repo.outbox))
		}
	}
}

// P9: an acquirer transport error leaves the intent pending and writes nothing.
func TestAuthorize_AcquirerError_LeavesPending(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	pi := seedIntent(t, repo, domain.PaymentIntentPending)

	_, err := newAuthorize(repo, &fakeAcquirer{wantFail: true}).Execute(context.Background(), pi.MerchantID, pi.ID)
	if err == nil || errors.Is(err, domain.ErrAcquirerRejected) {
		t.Fatalf("err = %v, want a transport error", err)
	}
	if got := repo.intents[pi.ID]; got.Status != domain.PaymentIntentPending || got.Version != 0 {
		t.Errorf("intent = %s v%d, want pending v0", got.Status, got.Version)
	}
	if len(repo.outbox) != 0 {
		t.Errorf("outbox = %d, want 0", len(repo.outbox))
	}
}

// P10: a commit failure after a decline leaves the intent pending; the retry
// sends the same idempotency key and persists the same decline.
func TestAuthorize_DeclineCommitFailure_RetryPersistsSameDecline(t *testing.T) {
	t.Parallel()
	base := newFakeRepo()
	repo := &failingUpdateRepo{fakeRepo: base, err: errors.New("commit failed"), failures: 1}
	acq := &fakeAcquirer{approve: false}
	pi := seedIntent(t, base, domain.PaymentIntentPending)
	uc := newAuthorize(repo, acq)

	if _, err := uc.Execute(context.Background(), pi.MerchantID, pi.ID); err == nil || errors.Is(err, domain.ErrAcquirerRejected) {
		t.Fatalf("first call err = %v, want the commit error", err)
	}
	if got := base.intents[pi.ID]; got.Status != domain.PaymentIntentPending || len(base.outbox) != 0 {
		t.Fatalf("after commit failure: status = %s outbox = %d, want pending and 0", got.Status, len(base.outbox))
	}

	if _, err := uc.Execute(context.Background(), pi.MerchantID, pi.ID); !errors.Is(err, domain.ErrAcquirerRejected) {
		t.Fatalf("retry err = %v, want ErrAcquirerRejected", err)
	}
	if len(acq.keys) != 2 || acq.keys[0] != acq.keys[1] || acq.keys[0] != pi.ID.String() {
		t.Errorf("idempotency keys = %v, want the intent id twice", acq.keys)
	}
	if got := base.intents[pi.ID]; got.Status != domain.PaymentIntentFailed || len(base.outbox) != 1 {
		t.Errorf("after retry: status = %s outbox = %d, want failed and 1", got.Status, len(base.outbox))
	}
}

// A lost version race returns ErrConcurrentUpdate and writes no outbox row.
func TestAuthorize_LostRace_ReturnsConcurrentUpdate(t *testing.T) {
	t.Parallel()
	base := newFakeRepo()
	repo := &failingUpdateRepo{fakeRepo: base, err: domain.ErrConcurrentUpdate, failures: 1}
	pi := seedIntent(t, base, domain.PaymentIntentPending)

	_, err := newAuthorize(repo, &fakeAcquirer{approve: true}).Execute(context.Background(), pi.MerchantID, pi.ID)
	if !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("err = %v, want ErrConcurrentUpdate", err)
	}
	if len(base.outbox) != 0 {
		t.Errorf("outbox = %d, want 0", len(base.outbox))
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

// P3, P8: capture of a pending or failed intent is rejected.
func TestCapture_FailsFromPendingOrFailed(t *testing.T) {
	t.Parallel()
	for _, status := range []domain.PaymentIntentStatus{domain.PaymentIntentPending, domain.PaymentIntentFailed} {
		repo := newFakeRepo()
		pi := seedIntent(t, repo, status)

		uc := application.NewCapturePaymentUseCase(repo, fixedClock{t: time.Now()}, &seqIDs{})
		_, err := uc.Execute(context.Background(), pi.MerchantID, pi.ID)
		if !errors.Is(err, domain.ErrInvalidStatusTransition) {
			t.Errorf("%s: err = %v, want ErrInvalidStatusTransition", status, err)
		}
		if len(repo.outbox) != 0 {
			t.Errorf("%s: outbox = %d, want 0", status, len(repo.outbox))
		}
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
		t.Errorf("outbox has an event on retry: capture is not idempotent")
	}
}

func TestAuthorize_OtherMerchant_NotFoundAndUntouched(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	acq := &fakeAcquirer{approve: true}
	pi := seedIntent(t, repo, domain.PaymentIntentPending)

	_, err := newAuthorize(repo, acq).Execute(context.Background(), uuid.New(), pi.ID)
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
