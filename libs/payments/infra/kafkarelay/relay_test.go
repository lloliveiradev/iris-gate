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

const (
	testTopic             = "payments.t1"
	testFailedTopic       = "payments.payment_intent.failed.v1"
	testMessageKey        = "k"
	testHeaderEventID     = "event-id"
	testHeaderEventType   = "event-type"
	testFailedPayload     = `{"failure_code":"card_declined","status":"failed"}`
	testPollInterval      = 10 * time.Millisecond
	testRunTimeout        = 200 * time.Millisecond
	testDefaultBatchLimit = 10
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
	// Remove marked messages from pending.
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
			Key:     testMessageKey,
			Payload: []byte(`{"i":` + string(rune('0'+i)) + `}`),
			Headers: map[string]string{testHeaderEventID: uuid.NewString()},
		}
	}
	return out
}

func TestRelay_PublishesAndMarks(t *testing.T) {
	t.Parallel()

	src := newFakeSource(mkPending(testTopic, 3)...)
	pub := &fakePublisher{}
	r := kafkarelay.New(src, pub, kafkarelay.Config{
		PollEvery:  testPollInterval,
		BatchLimit: testDefaultBatchLimit,
	})

	ctx, cancel := context.WithTimeout(context.Background(), testRunTimeout)
	defer cancel()
	_ = r.Run(ctx)

	src.mu.Lock()
	defer src.mu.Unlock()
	if len(src.pending) != 0 {
		t.Errorf("still have %d unpublished messages", len(src.pending))
	}
	if len(src.published) != 3 {
		t.Errorf("published count = %d, want 3", len(src.published))
	}
	pub.mu.Lock()
	if len(pub.sent) == 0 {
		t.Fatalf("expected at least one Publish call")
	}
	pub.mu.Unlock()
}

func TestRelay_PublishFailure_Retries(t *testing.T) {
	t.Parallel()

	src := newFakeSource(mkPending(testTopic, 2)...)
	pub := &fakePublisher{failNext: true}
	r := kafkarelay.New(src, pub, kafkarelay.Config{
		PollEvery:  testPollInterval,
		BatchLimit: testDefaultBatchLimit,
	})

	ctx, cancel := context.WithTimeout(context.Background(), testRunTimeout)
	defer cancel()
	_ = r.Run(ctx)

	// After the first failure, the next iteration retries and succeeds.
	src.mu.Lock()
	defer src.mu.Unlock()
	if len(src.pending) != 0 {
		t.Errorf("retry did not clear pending messages: %d remaining", len(src.pending))
	}
}

func TestRelay_PublishesFailedEventWithHeaders(t *testing.T) {
	t.Parallel()

	msgID := uuid.New()
	eventID := uuid.NewString()
	pending := kafkarelay.PendingMessage{
		ID:      msgID,
		Topic:   testFailedTopic,
		Key:     testMessageKey,
		Payload: []byte(testFailedPayload),
		Headers: map[string]string{
			testHeaderEventID:   eventID,
			testHeaderEventType: testFailedTopic,
		},
	}

	src := newFakeSource(pending)
	pub := &fakePublisher{}
	r := kafkarelay.New(src, pub, kafkarelay.Config{
		PollEvery:  testPollInterval,
		BatchLimit: testDefaultBatchLimit,
	})

	ctx, cancel := context.WithTimeout(context.Background(), testRunTimeout)
	defer cancel()
	_ = r.Run(ctx)

	src.mu.Lock()
	if len(src.pending) != 0 {
		t.Errorf("still have %d unpublished messages", len(src.pending))
	}
	if _, ok := src.published[msgID]; !ok {
		t.Errorf("expected message %s to be marked published", msgID)
	}
	src.mu.Unlock()

	pub.mu.Lock()
	defer pub.mu.Unlock()
	if len(pub.sent) != 1 || len(pub.sent[0]) != 1 {
		t.Fatalf("published batches = %d, want 1 batch with 1 message", len(pub.sent))
	}
	got := pub.sent[0][0]
	if got.Topic != testFailedTopic {
		t.Errorf("topic = %q, want %q", got.Topic, testFailedTopic)
	}
	if string(got.Key) != testMessageKey {
		t.Errorf("key = %q, want %q", string(got.Key), testMessageKey)
	}
	if string(got.Value) != testFailedPayload {
		t.Errorf("value = %q, want %q", string(got.Value), testFailedPayload)
	}

	gotHeaders := make(map[string]string, len(got.Headers))
	for _, h := range got.Headers {
		gotHeaders[h.Key] = string(h.Value)
	}
	if gotHeaders[testHeaderEventID] != eventID {
		t.Errorf("header %s = %q, want %q", testHeaderEventID, gotHeaders[testHeaderEventID], eventID)
	}
	if gotHeaders[testHeaderEventType] != testFailedTopic {
		t.Errorf("header %s = %q, want %q", testHeaderEventType, gotHeaders[testHeaderEventType], testFailedTopic)
	}
}
