// Package pgstore implementa PaymentIntentRepo + IdempotencyStore (port do
// middleware HTTP) sobre Postgres usando pgx v5.
package pgstore

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/domain"
	"github.com/iris-gate/iris-gate/libs/payments/infra/idempotency"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// CreateWithOutbox abre uma transação e escreve o intent + a mensagem de
// outbox atomicamente — invariante do Outbox pattern.
func (r *Repo) CreateWithOutbox(ctx context.Context, p domain.PaymentIntent, ev application.OutboxMessage) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := insertPaymentIntent(ctx, tx, p); err != nil {
			return err
		}
		return insertOutbox(ctx, tx, ev)
	})
}

func insertPaymentIntent(ctx context.Context, db DBTX, p domain.PaymentIntent) error {
	const q = `
		INSERT INTO payment_intents
			(id, merchant_id, amount_cents, currency, status, card_token, description, created_at, authorized_at, captured_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`
	_, err := db.Exec(ctx, q,
		p.ID, p.MerchantID, p.Amount.AmountCents, p.Amount.Currency,
		string(p.Status), p.CardToken, p.Description, p.CreatedAt, p.AuthorizedAt, p.CapturedAt,
	)
	return err
}

func insertOutbox(ctx context.Context, db DBTX, ev application.OutboxMessage) error {
	const q = `
		INSERT INTO outbox_messages (topic, msg_key, payload, headers)
		VALUES ($1,$2,$3,$4)
	`
	headers := ev.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	_, err := db.Exec(ctx, q, ev.Topic, ev.Key, ev.Payload, headers)
	return err
}

// GetByID lê um intent.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (domain.PaymentIntent, error) {
	const q = `
		SELECT id, merchant_id, amount_cents, currency, status, card_token, description,
		       created_at, authorized_at, captured_at
		FROM payment_intents WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, q, id)
	return scanIntent(row)
}

// GetByMerchantAndIdempotencyKey existe pra simetria com o port — a checagem
// de idempotência real acontece no middleware contra `idempotency_records`,
// não na tabela payment_intents.
func (r *Repo) GetByMerchantAndIdempotencyKey(_ context.Context, _ uuid.UUID, _ string) (domain.PaymentIntent, error) {
	return domain.PaymentIntent{}, domain.ErrPaymentIntentNotFound
}

func scanIntent(row pgx.Row) (domain.PaymentIntent, error) {
	var p domain.PaymentIntent
	var status, currency string
	var amount int64
	if err := row.Scan(&p.ID, &p.MerchantID, &amount, &currency, &status, &p.CardToken, &p.Description,
		&p.CreatedAt, &p.AuthorizedAt, &p.CapturedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PaymentIntent{}, domain.ErrPaymentIntentNotFound
		}
		return domain.PaymentIntent{}, err
	}
	p.Status = domain.PaymentIntentStatus(status)
	money, err := domain.NewMoney(amount, currency)
	if err != nil {
		return domain.PaymentIntent{}, fmt.Errorf("scan money: %w", err)
	}
	p.Amount = money
	return p, nil
}

// ---------- Outbox: relay worker (stub) ----------

// PullUnpublished devolve até `limit` mensagens não publicadas em ordem
// FIFO. Relay worker real (push pra Kafka) entra em PR subsequente; por ora
// o método está pronto pra ser plugado.
func (r *Repo) PullUnpublished(ctx context.Context, limit int) ([]application.OutboxMessage, error) {
	const q = `
		SELECT topic, msg_key, payload, headers
		FROM outbox_messages
		WHERE published_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []application.OutboxMessage
	for rows.Next() {
		var msg application.OutboxMessage
		var headers map[string]string
		if err := rows.Scan(&msg.Topic, &msg.Key, &msg.Payload, &headers); err != nil {
			return nil, err
		}
		msg.Headers = headers
		out = append(out, msg)
	}
	return out, rows.Err()
}

// ---------- Idempotency store ----------

type IdempotencyStore struct{ pool *pgxpool.Pool }

func NewIdempotencyStore(pool *pgxpool.Pool) *IdempotencyStore { return &IdempotencyStore{pool: pool} }

// Existing devolve um registro armazenado pra (scope, key). Se não existir,
// devolve (nil, nil) — chamador decide o que fazer.
func (s *IdempotencyStore) Existing(ctx context.Context, scope, key string) (*idempotency.Record, error) {
	const q = `
		SELECT request_hash, response_status, response_body
		FROM idempotency_records
		WHERE scope = $1 AND key = $2
	`
	var rec idempotency.Record
	if err := s.pool.QueryRow(ctx, q, scope, key).Scan(&rec.RequestHash, &rec.ResponseStatus, &rec.ResponseBody); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// Save insere o resultado de uma operação. Retorna (alreadyExists=true) quando
// outra request paralela já gravou — chamador deve consultar Existing.
func (s *IdempotencyStore) Save(ctx context.Context, scope, key string, requestHash []byte, status int, body []byte) (alreadyExists bool, err error) {
	const q = `
		INSERT INTO idempotency_records (scope, key, request_hash, response_status, response_body)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (scope, key) DO NOTHING
	`
	tag, err := s.pool.Exec(ctx, q, scope, key, requestHash, status, body)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 0, nil
}

// ---------- Migrations ----------

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrateEmbedded aplica as migrations deste contexto na ordem.
func MigrateEmbedded(ctx context.Context, db DBTX) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("init schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&exists); err != nil {
			return fmt.Errorf("check %s: %w", name, err)
		}
		if exists {
			continue
		}
		body, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := db.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := db.Exec(ctx, `INSERT INTO schema_migrations(name) VALUES ($1)`, name); err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
	}
	return nil
}

// SinceEpoch é um helper redundante mas explícito pro time-travel em testes.
func SinceEpoch(t time.Time) int64 { return t.Unix() }
