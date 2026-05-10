// Package kafkarelay implementa o Outbox Relay Worker (ADR-0002):
// poll outbox_messages → publish em Kafka → mark published_at.
//
// Não há "exactly-once" — o worker pode duplicar quando crashar entre
// publish e mark_published; consumidores devem ser idempotentes.
package kafkarelay

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// Source é o port de origem das mensagens — implementado por
// payments/infra/pgstore.Repo.
type Source interface {
	PullUnpublished(ctx context.Context, limit int) ([]PendingMessage, error)
	MarkPublished(ctx context.Context, ids []uuid.UUID, at time.Time) error
}

// PendingMessage é o shape esperado pelo Source. Replicado aqui pra evitar
// dep cíclica com pgstore.
type PendingMessage struct {
	ID      uuid.UUID
	Topic   string
	Key     string
	Payload []byte
	Headers map[string]string
}

// Publisher é o port pra Kafka — útil pra testar com fake.
type Publisher interface {
	Publish(ctx context.Context, msgs []kafka.Message) error
	Close() error
}

// KafkaWriter envolve um *kafka.Writer único pra publicar em qualquer tópico.
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

// Relay é o worker em si. Use Run pra subir o loop em background.
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

// Run bloqueia até ctx.Done(). Erros transitórios são logados e o loop continua.
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
		// publicado mas não marcado: vai duplicar no próximo poll. Aceito —
		// consumidores são idempotentes (conforme ADR-0002).
		return err
	}
	slog.LogAttrs(ctx, slog.LevelInfo, "kafkarelay.published", slog.Int("n", len(ids)))
	return nil
}
