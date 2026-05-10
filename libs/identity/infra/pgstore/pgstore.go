// Package pgstore implementa MerchantRepo + APIKeyRepo (ports da application
// layer) sobre Postgres usando pgx v5.
//
// Decisão (2026-05-10): hand-roll inicial. Architecture.md prevê sqlc como alvo;
// migração para sqlc é trivial quando o volume de queries justificar. Fonte:
// docs/roadmap.md (Fase 2 — deltas).
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

	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

// DBTX é a interface que pgxpool.Pool e pgx.Tx ambos satisfazem — útil pra
// cabear o repo dentro de uma transação do uso futuro de outbox.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repo struct{ db DBTX }

func New(pool *pgxpool.Pool) *Repo { return &Repo{db: pool} }

// ---------- Merchants ----------

func (r *Repo) Create(ctx context.Context, m domain.Merchant) error {
	const q = `
		INSERT INTO merchants (id, legal_name, document, contact_email, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`
	_, err := r.db.Exec(ctx, q, m.ID, m.LegalName, m.Document, m.ContactEmail, string(m.Status), m.CreatedAt, m.UpdatedAt)
	if isUniqueViolation(err, "merchants_document_key") {
		return domain.ErrDocumentExists
	}
	return err
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (domain.Merchant, error) {
	const q = `
		SELECT id, legal_name, document, contact_email, status, created_at, updated_at
		FROM merchants WHERE id = $1
	`
	row := r.db.QueryRow(ctx, q, id)
	return scanMerchant(row)
}

func (r *Repo) GetByDocument(ctx context.Context, doc string) (domain.Merchant, error) {
	const q = `
		SELECT id, legal_name, document, contact_email, status, created_at, updated_at
		FROM merchants WHERE document = $1
	`
	row := r.db.QueryRow(ctx, q, doc)
	return scanMerchant(row)
}

func scanMerchant(row pgx.Row) (domain.Merchant, error) {
	var m domain.Merchant
	var status string
	if err := row.Scan(&m.ID, &m.LegalName, &m.Document, &m.ContactEmail, &status, &m.CreatedAt, &m.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Merchant{}, domain.ErrMerchantNotFound
		}
		return domain.Merchant{}, err
	}
	m.Status = domain.MerchantStatus(status)
	return m, nil
}

// ---------- API keys ----------

func (r *Repo) CreateAPIKey(ctx context.Context, k domain.APIKey) error {
	const q = `
		INSERT INTO api_keys (id, merchant_id, prefix, hash, status, scopes, created_at, last_used_at, revoked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`
	_, err := r.db.Exec(ctx, q, k.ID, k.MerchantID, k.Prefix, k.Hash, string(k.Status), k.Scopes, k.CreatedAt, k.LastUsedAt, k.RevokedAt)
	return err
}

func (r *Repo) GetAPIKeyByPrefix(ctx context.Context, prefix string) (domain.APIKey, error) {
	const q = `
		SELECT id, merchant_id, prefix, hash, status, scopes, created_at, last_used_at, revoked_at
		FROM api_keys WHERE prefix = $1
	`
	row := r.db.QueryRow(ctx, q, prefix)
	return scanAPIKey(row)
}

func (r *Repo) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (domain.APIKey, error) {
	const q = `
		SELECT id, merchant_id, prefix, hash, status, scopes, created_at, last_used_at, revoked_at
		FROM api_keys WHERE id = $1
	`
	row := r.db.QueryRow(ctx, q, id)
	return scanAPIKey(row)
}

func (r *Repo) RevokeAPIKey(ctx context.Context, id uuid.UUID, at time.Time) (domain.APIKey, error) {
	const q = `
		UPDATE api_keys
		SET status = 'revoked', revoked_at = $2
		WHERE id = $1 AND status = 'active'
		RETURNING id, merchant_id, prefix, hash, status, scopes, created_at, last_used_at, revoked_at
	`
	row := r.db.QueryRow(ctx, q, id, at)
	k, err := scanAPIKey(row)
	if errors.Is(err, domain.ErrAPIKeyNotFound) {
		// Pode ser "not found" OU "já revogada". Distinguir consultando.
		if existing, gerr := r.GetAPIKeyByID(ctx, id); gerr == nil {
			return existing, nil
		}
	}
	return k, err
}

func (r *Repo) TouchAPIKeyLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	const q = `UPDATE api_keys SET last_used_at = $2 WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id, at)
	return err
}

func scanAPIKey(row pgx.Row) (domain.APIKey, error) {
	var k domain.APIKey
	var status string
	if err := row.Scan(&k.ID, &k.MerchantID, &k.Prefix, &k.Hash, &status, &k.Scopes, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.APIKey{}, domain.ErrAPIKeyNotFound
		}
		return domain.APIKey{}, err
	}
	k.Status = domain.APIKeyStatus(status)
	return k, nil
}

// ---------- Wiring para conformar com os ports da application layer ----------

// MerchantsRepo expõe métodos do Repo no formato esperado por application.MerchantRepo.
type MerchantsRepo struct{ r *Repo }

func (m MerchantsRepo) Create(ctx context.Context, x domain.Merchant) error {
	return m.r.Create(ctx, x)
}

func (m MerchantsRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Merchant, error) {
	return m.r.GetByID(ctx, id)
}

func (m MerchantsRepo) GetByDocument(ctx context.Context, doc string) (domain.Merchant, error) {
	return m.r.GetByDocument(ctx, doc)
}

// APIKeysRepo expõe métodos do Repo no formato esperado por application.APIKeyRepo.
type APIKeysRepo struct{ r *Repo }

func (k APIKeysRepo) Create(ctx context.Context, x domain.APIKey) error {
	return k.r.CreateAPIKey(ctx, x)
}

func (k APIKeysRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.APIKey, error) {
	return k.r.GetAPIKeyByID(ctx, id)
}

func (k APIKeysRepo) GetByPrefix(ctx context.Context, prefix string) (domain.APIKey, error) {
	return k.r.GetAPIKeyByPrefix(ctx, prefix)
}

func (k APIKeysRepo) Revoke(ctx context.Context, id uuid.UUID, at time.Time) (domain.APIKey, error) {
	return k.r.RevokeAPIKey(ctx, id, at)
}

func (k APIKeysRepo) TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	return k.r.TouchAPIKeyLastUsed(ctx, id, at)
}

// Repos é uma conveniência pra obter ambos os adapters de uma vez.
func (r *Repo) Repos() (MerchantsRepo, APIKeysRepo) {
	return MerchantsRepo{r}, APIKeysRepo{r}
}

// ---------- Helpers ----------

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// ---------- Migrations ----------

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrateEmbedded aplica todas as migrations embarcadas no diretório migrations/
// dentro deste pacote. Idempotente: usa uma tabela schema_migrations simples.
//
// Em prod a recomendação é usar atlas/golang-migrate; este helper é o suficiente
// pra dev local + testes de integração.
func MigrateEmbedded(ctx context.Context, db DBTX) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("init schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
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
