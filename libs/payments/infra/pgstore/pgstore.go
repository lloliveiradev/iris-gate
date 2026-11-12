// Package pgstore implements PaymentIntentRepo and the idempotency Store (the
// HTTP middleware port) on Postgres with pgx v5.
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
	"github.com/iris-gate/iris-gate/libs/payments/infra/kafkarelay"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// CreateWithOutbox writes the intent and its outbox message in one
// transaction (Outbox pattern invariant).
func (r *Repo) CreateWithOutbox(ctx context.Context, p domain.PaymentIntent, ev application.OutboxMessage) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := insertPaymentIntent(ctx, tx, p); err != nil {
			return err
		}
		return insertOutbox(ctx, tx, ev)
	})
}

// UpdateWithOutbox updates the intent status and timestamps, scoped to its
// merchant, and writes the transition event to the outbox in the same tx.
func (r *Repo) UpdateWithOutbox(ctx context.Context, p domain.PaymentIntent, ev application.OutboxMessage) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		const q = `
			UPDATE payment_intents
			SET status = $2, authorized_at = $3, captured_at = $4
			WHERE id = $1 AND merchant_id = $5
		`
		tag, err := tx.Exec(ctx, q, p.ID, string(p.Status), p.AuthorizedAt, p.CapturedAt, p.MerchantID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrPaymentIntentNotFound
		}
		return insertOutbox(ctx, tx, ev)
	})
}

// MarkPublished is called by the relay worker once the broker has
// confirmed the event.
func (r *Repo) MarkPublished(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	const q = `UPDATE outbox_messages SET published_at = $2 WHERE id = ANY($1)`
	_, err := r.pool.Exec(ctx, q, ids, at)
	return err
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

// GetByID loads an intent owned by merchantID. Another merchant's intent
// yields ErrPaymentIntentNotFound, the same as a missing id.
func (r *Repo) GetByID(ctx context.Context, merchantID, id uuid.UUID) (domain.PaymentIntent, error) {
	const q = `
		SELECT id, merchant_id, amount_cents, currency, status, card_token, description,
		       created_at, authorized_at, captured_at
		FROM payment_intents WHERE id = $1 AND merchant_id = $2
	`
	row := r.pool.QueryRow(ctx, q, id, merchantID)
	return scanIntent(row)
}

// ListByMerchant returns the merchant's intents, newest first.
func (r *Repo) ListByMerchant(ctx context.Context, merchantID uuid.UUID, limit int) ([]domain.PaymentIntent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
		SELECT id, merchant_id, amount_cents, currency, status, card_token, description,
		       created_at, authorized_at, captured_at
		FROM payment_intents
		WHERE merchant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, q, merchantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PaymentIntent
	for rows.Next() {
		p, scanErr := scanIntent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, p)
	}
	return out, rows.Err()
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

// PullUnpublished returns up to `limit` unpublished messages in FIFO order.
// Implementa kafkarelay.Source.
func (r *Repo) PullUnpublished(ctx context.Context, limit int) ([]kafkarelay.PendingMessage, error) {
	const q = `
		SELECT id, topic, msg_key, payload, headers
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
	var out []kafkarelay.PendingMessage
	for rows.Next() {
		var pm kafkarelay.PendingMessage
		var headers map[string]string
		if err := rows.Scan(&pm.ID, &pm.Topic, &pm.Key, &pm.Payload, &headers); err != nil {
			return nil, err
		}
		pm.Headers = headers
		out = append(out, pm)
	}
	return out, rows.Err()
}

// ---------- Idempotency store ----------

type IdempotencyStore struct{ pool *pgxpool.Pool }

func NewIdempotencyStore(pool *pgxpool.Pool) *IdempotencyStore { return &IdempotencyStore{pool: pool} }

// Existing returns the stored record for (merchantID, scope, key), or
// (nil, nil) when there is none. Records are never shared across merchants.
func (s *IdempotencyStore) Existing(ctx context.Context, merchantID uuid.UUID, scope, key string) (*idempotency.Record, error) {
	const q = `
		SELECT request_hash, response_status, response_body
		FROM idempotency_records
		WHERE merchant_id = $1 AND scope = $2 AND key = $3
	`
	var rec idempotency.Record
	if err := s.pool.QueryRow(ctx, q, merchantID, scope, key).Scan(&rec.RequestHash, &rec.ResponseStatus, &rec.ResponseBody); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// Save stores the result of an operation for (merchantID, scope, key). It
// returns alreadyExists=true when a parallel request stored it first.
func (s *IdempotencyStore) Save(ctx context.Context, merchantID uuid.UUID, scope, key string, requestHash []byte, status int, body []byte) (alreadyExists bool, err error) {
	const q = `
		INSERT INTO idempotency_records (merchant_id, scope, key, request_hash, response_status, response_body)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (merchant_id, scope, key) DO NOTHING
	`
	tag, err := s.pool.Exec(ctx, q, merchantID, scope, key, requestHash, status, body)
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
