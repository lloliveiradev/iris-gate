package application_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

type fakeRepo struct {
	mu      sync.Mutex
	intents map[uuid.UUID]domain.PaymentIntent
	outbox  []application.OutboxMessage
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{intents: map[uuid.UUID]domain.PaymentIntent{}}
}

func (r *fakeRepo) CreateWithOutbox(_ context.Context, p domain.PaymentIntent, ev application.OutboxMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.intents[p.ID] = p
	r.outbox = append(r.outbox, ev)
	return nil
}

func (r *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (domain.PaymentIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.intents[id]
	if !ok {
		return domain.PaymentIntent{}, domain.ErrPaymentIntentNotFound
	}
	return p, nil
}

func (r *fakeRepo) GetByMerchantAndIdempotencyKey(_ context.Context, _ uuid.UUID, _ string) (domain.PaymentIntent, error) {
	return domain.PaymentIntent{}, domain.ErrPaymentIntentNotFound
}

func (r *fakeRepo) ListByMerchant(_ context.Context, merchantID uuid.UUID, limit int) ([]domain.PaymentIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.PaymentIntent, 0)
	for _, p := range r.intents {
		if p.MerchantID == merchantID {
			out = append(out, p)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type seqIDs struct {
	mu sync.Mutex
	n  int
}

func (g *seqIDs) NewUUID() uuid.UUID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return uuid.NewMD5(uuid.NameSpaceOID, []byte(strconv.Itoa(g.n)))
}

func TestCreatePaymentIntent_OK(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	clock := fixedClock{t: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)}
	uc := application.NewCreatePaymentIntentUseCase(repo, clock, &seqIDs{})

	amt, _ := domain.NewMoney(12990, "BRL")
	pi, err := uc.Execute(context.Background(), application.CreatePaymentIntentInput{
		MerchantID:  uuid.New(),
		Amount:      amt,
		CardToken:   "tok_abcdef12", // gitleaks:allow fixture: not a real card token
		Description: "ACME order",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if pi.Status != domain.PaymentIntentPending {
		t.Errorf("status = %v", pi.Status)
	}
	if got := repo.intents[pi.ID]; got.ID != pi.ID {
		t.Errorf("intent not persisted")
	}
	if n := len(repo.outbox); n != 1 {
		t.Fatalf("outbox messages = %d, want 1", n)
	}
	if topic := repo.outbox[0].Topic; topic != application.TopicPaymentIntentCreatedV1 {
		t.Errorf("topic = %q", topic)
	}
}

func TestCreatePaymentIntent_ValidationFails(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	uc := application.NewCreatePaymentIntentUseCase(repo, fixedClock{t: time.Now()}, &seqIDs{})

	amt, _ := domain.NewMoney(100, "BRL")
	_, err := uc.Execute(context.Background(), application.CreatePaymentIntentInput{
		MerchantID: uuid.Nil, // invalid
		Amount:     amt,
		CardToken:  "tok_abcdef12", // gitleaks:allow fixture: not a real card token
	})
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if len(repo.outbox) != 0 {
		t.Errorf("outbox has messages on an invalid flow")
	}
}
