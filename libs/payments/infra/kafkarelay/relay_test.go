package kafkarelay_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"github.com/iris-gate/iris-gate/libs/payments/infra/kafkarelay"
)

type fakeSource struct {
	mu        sync.Mutex
	pending   []kafkarelay.PendingMessage
	published map[uuid.UUID]time.Time
}

func newFakeSource(initial ...kafkarelay.PendingMessage) *fakeSource {
	return &fakeSource{
		pending:   append([]kafkarelay.PendingMessage{}, initial...),
		published: map[uuid.UUID]time.Time{},
	}
}

func (s *fakeSource) PullUnpublished(_ context.Context, limit int) ([]kafkarelay.PendingMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) == 0 {
		return nil, nil
	}
	if limit > len(s.pending) {
		limit = len(s.pending)
	}
	out := append([]kafkarelay.PendingMessage{}, s.pending[:limit]...)
	return out, nil
}

func (s *fakeSource) MarkPublished(_ context.Context, ids []uuid.UUID, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		s.published[id] = at
	}
	// remove os marcados de pending
	keep := s.pending[:0]
	for _, p := range s.pending {
		if _, done := s.published[p.ID]; !done {
			keep = append(keep, p)
		}
	}
	s.pending = keep
	return nil
}

type fakePublisher struct {
	mu       sync.Mutex
	sent     [][]kafka.Message
	failNext bool
}

func (p *fakePublisher) Publish(_ context.Context, msgs []kafka.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failNext {
		p.failNext = false
		return errors.New("transient publish failure")
	}
	p.sent = append(p.sent, msgs)
	return nil
}

func (p *fakePublisher) Close() error { return nil }

func mkPending(topic string, n int) []kafkarelay.PendingMessage {
	out := make([]kafkarelay.PendingMessage, n)
	for i := range n {
		out[i] = kafkarelay.PendingMessage{
			ID:      uuid.New(),
			Topic:   topic,
			Key:     "k",
			Payload: []byte(`{"i":` + string(rune('0'+i)) + `}`),
			Headers: map[string]string{"event-id": uuid.NewString()},
		}
	}
	return out
}

func TestRelay_PublishesAndMarks(t *testing.T) {
	t.Parallel()

	src := newFakeSource(mkPending("payments.t1", 3)...)
	pub := &fakePublisher{}
	r := kafkarelay.New(src, pub, kafkarelay.Config{
		PollEvery:  10 * time.Millisecond,
		BatchLimit: 10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)

	src.mu.Lock()
	defer src.mu.Unlock()
	if len(src.pending) != 0 {
		t.Errorf("ainda restam %d mensagens não publicadas", len(src.pending))
	}
	if len(src.published) != 3 {
		t.Errorf("published count = %d, want 3", len(src.published))
	}
	pub.mu.Lock()
	if len(pub.sent) == 0 {
		t.Fatalf("nenhuma chamada Publish")
	}
	pub.mu.Unlock()
}

func TestRelay_PublishFailure_Retries(t *testing.T) {
	t.Parallel()

	src := newFakeSource(mkPending("payments.t1", 2)...)
	pub := &fakePublisher{failNext: true}
	r := kafkarelay.New(src, pub, kafkarelay.Config{
		PollEvery:  10 * time.Millisecond,
		BatchLimit: 10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = r.Run(ctx)

	// Após primeira falha, próxima iteração tenta de novo e sucede.
	src.mu.Lock()
	defer src.mu.Unlock()
	if len(src.pending) != 0 {
		t.Errorf("retry não limpou: %d ainda pendentes", len(src.pending))
	}
}
