// Package kafkarelay implements the Outbox Relay Worker (ADR-0002):
// poll outbox_messages → publish to Kafka → mark published_at.
//
// Delivery is at-least-once, not exactly-once: the worker may duplicate
// messages if it crashes between publish and mark_published; consumers must be
// idempotent.
package kafkarelay

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// Source is the message origin port, implemented by
// payments/infra/pgstore.Repo.
type Source interface {
	PullUnpublished(ctx context.Context, limit int) ([]PendingMessage, error)
	MarkPublished(ctx context.Context, ids []uuid.UUID, at time.Time) error
}

// PendingMessage is the shape expected from Source, duplicated here to avoid a
// cyclic dependency with pgstore.
type PendingMessage struct {
	ID      uuid.UUID
	Topic   string
	Key     string
	Payload []byte
	Headers map[string]string
}

// Publisher is the Kafka publishing port, replaceable with a fake in tests.
type Publisher interface {
	Publish(ctx context.Context, msgs []kafka.Message) error
	Close() error
}

// KafkaWriter wraps a single *kafka.Writer that can publish to any topic.
type KafkaWriter struct{ w *kafka.Writer }

func NewKafkaWriter(brokers []string) *KafkaWriter {
	return &KafkaWriter{
		w: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Balancer:               &kafka.Hash{},
			AllowAutoTopicCreation: false,
			RequiredAcks:           kafka.RequireAll,
			BatchTimeout:           50 * time.Millisecond,
		},
	}
}

func (k *KafkaWriter) Publish(ctx context.Context, msgs []kafka.Message) error {
	return k.w.WriteMessages(ctx, msgs...)
}

func (k *KafkaWriter) Close() error { return k.w.Close() }

// Relay is the worker itself. Call Run to start the polling loop in the background.
type Relay struct {
	src        Source
	pub        Publisher
	now        func() time.Time
	pollEvery  time.Duration
	batchLimit int
}

type Config struct {
	PollEvery  time.Duration // default 250ms
	BatchLimit int           // default 100
	Now        func() time.Time
}

func New(src Source, pub Publisher, cfg Config) *Relay {
	if cfg.PollEvery == 0 {
		cfg.PollEvery = 250 * time.Millisecond
	}
	if cfg.BatchLimit == 0 {
		cfg.BatchLimit = 100
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Relay{src: src, pub: pub, now: cfg.Now, pollEvery: cfg.PollEvery, batchLimit: cfg.BatchLimit}
}

// Run blocks until ctx.Done(). Transient errors are logged and the loop continues.
func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.pump(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.LogAttrs(ctx, slog.LevelError, "kafkarelay.pump", slog.Any("err", err))
			}
		}
	}
}

func (r *Relay) pump(ctx context.Context) error {
	pending, err := r.src.PullUnpublished(ctx, r.batchLimit)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	msgs := make([]kafka.Message, 0, len(pending))
	for _, p := range pending {
		hdrs := make([]kafka.Header, 0, len(p.Headers))
		for k, v := range p.Headers {
			hdrs = append(hdrs, kafka.Header{Key: k, Value: []byte(v)})
		}
		msgs = append(msgs, kafka.Message{
			Topic:   p.Topic,
			Key:     []byte(p.Key),
			Value:   p.Payload,
			Headers: hdrs,
		})
	}

	if err := r.pub.Publish(ctx, msgs); err != nil {
		return err
	}

	ids := make([]uuid.UUID, 0, len(pending))
	for _, p := range pending {
		ids = append(ids, p.ID)
	}
	if err := r.src.MarkPublished(ctx, ids, r.now()); err != nil {
		// Published to Kafka but not marked: the next poll will republish.
		// Accepted because consumers are idempotent (per ADR-0002).
		return err
	}
	slog.LogAttrs(ctx, slog.LevelInfo, "kafkarelay.published", slog.Int("n", len(ids)))
	return nil
}
