//go:build integration

// Integration tests for the phase 08 identity migrations (0006 to 0009, plan
// tests M1 to M4). Each test runs in a schema of its own, created for it and
// dropped afterwards, so it starts empty even when DATABASE_URL points at a
// database that `make migrate` already brought up to date. The role that
// DATABASE_URL names must be able to create a schema (the bootstrap superuser
// or iris_migrator). Without DATABASE_URL the tests skip.
package pgstore_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iris-gate/iris-gate/libs/identity/domain"
	"github.com/iris-gate/iris-gate/libs/identity/infra/pgstore"
)

const (
	databaseURLEnv     = "DATABASE_URL"
	integrationTimeout = 30 * time.Second
	// schemaPrefix names the throwaway schema of each test.
	schemaPrefix = "identity_migrations_"
	// createSchemaMigrationsSQL is the bookkeeping table as MigrateEmbedded
	// creates it.
	createSchemaMigrationsSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`

	sqlStateUniqueViolation = "23505"
	sqlStateCheckViolation  = "23514"

	oneOwnerIndex        = "merchant_memberships_one_owner_idx"
	hashMatchesKindCheck = "api_keys_hash_matches_kind_check"

	backfilledKind = "secret"
	backfilledName = "legacy"
)

// Fake values. None is a real credential.
const (
	fixtureLegalName    = "Fixture Merchant Ltda"
	fixtureDocument     = "00000000000191"
	fixtureContactEmail = "merchant@example.com"
	fixtureKeyHash      = "fixture-argon2id-phc-hash"
	fixtureKeyPrefix    = "irk_fixture"
	fixtureOwnerEmail   = "owner@example.com"
	fixtureSecondEmail  = "second@example.com"
)

// phase07Migrations are the identity migrations a phase 07 database has.
var phase07Migrations = []string{"0001_init.sql"}

// identityTables are the tables 0006 to 0009 create.
var identityTables = []string{
	"users", "refresh_tokens", "user_tokens",
	"merchant_memberships", "invitations", "platform_staff",
	"audit_events",
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), integrationTimeout)
	t.Cleanup(cancel)
	return ctx
}

// newSchemaPool creates an empty schema and returns a pool whose search_path
// is that schema, then public (where 00-extensions.sql installs citext). The
// schema is dropped when the test ends.
func newSchemaPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv(databaseURLEnv)
	if url == "" {
		t.Skipf("%s not set", databaseURLEnv)
	}
	schema := fmt.Sprintf("%s%d", schemaPrefix, time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+quoted); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
		defer cancel()
		if _, err := admin.Exec(cleanupCtx, `DROP SCHEMA `+quoted+` CASCADE`); err != nil {
			t.Errorf("drop schema: %v", err)
		}
		_ = admin.Close(cleanupCtx)
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse %s: %v", databaseURLEnv, err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ", public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// migrateToPhase07 applies the identity migrations of a phase 07 database and
// records them as MigrateEmbedded does, so MigrateEmbedded later applies only
// what came after them.
func migrateToPhase07(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, createSchemaMigrationsSQL); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	for _, name := range phase07Migrations {
		body, err := fs.ReadFile(os.DirFS("migrations"), name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			t.Fatalf("record %s: %v", name, err)
		}
	}
}

func migrateAll(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if err := pgstore.MigrateEmbedded(ctx, pool); err != nil {
		t.Fatalf("MigrateEmbedded: %v", err)
	}
}

func appliedMigrations(ctx context.Context, t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT name FROM schema_migrations ORDER BY name`)
	if err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("scan schema_migrations: %v", err)
	}
	return names
}

func embeddedMigrations(t *testing.T) []string {
	t.Helper()
	entries, err := fs.ReadDir(os.DirFS("."), "migrations")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names
}

// seedMerchant inserts a merchant through the phase 07 repository method.
func seedMerchant(ctx context.Context, t *testing.T, repo *pgstore.Repo) uuid.UUID {
	t.Helper()
	now := time.Now().UTC()
	m := domain.Merchant{
		ID:           uuid.New(),
		LegalName:    fixtureLegalName,
		Document:     fixtureDocument,
		ContactEmail: fixtureContactEmail,
		Status:       domain.MerchantActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("seed merchant: %v", err)
	}
	return m.ID
}

// seedAPIKey inserts a key through the phase 07 repository method, which
// writes neither kind nor name.
func seedAPIKey(ctx context.Context, t *testing.T, repo *pgstore.Repo, merchantID uuid.UUID, prefix string, status domain.APIKeyStatus) uuid.UUID {
	t.Helper()
	k := domain.APIKey{
		ID:         uuid.New(),
		MerchantID: merchantID,
		Prefix:     prefix,
		Hash:       fixtureKeyHash,
		Status:     status,
		Scopes:     []string{},
		CreatedAt:  time.Now().UTC(),
	}
	if status == domain.APIKeyRevoked {
		k.RevokedAt = &k.CreatedAt
	}
	if err := repo.CreateAPIKey(ctx, k); err != nil {
		t.Fatalf("seed api key %s: %v", prefix, err)
	}
	return k.ID
}

func seedUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (email) VALUES ($1) RETURNING id`, email).Scan(&id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func requireViolation(t *testing.T, what string, err error, code, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("%s: want SQLSTATE %s on %s, got %v", what, code, constraint, err)
	}
	if pgErr.Code != code || pgErr.ConstraintName != constraint {
		t.Fatalf("%s: got SQLSTATE %s on %q, want %s on %q", what, pgErr.Code, pgErr.ConstraintName, code, constraint)
	}
}

// requireAuditEventIDIsUUID checks audit_events.id as the database reports it:
// type uuid, with a default, as AuditEvent.id is in the 08.1 contract.
func requireAuditEventIDIsUUID(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var dataType string
	var columnDefault *string
	err := pool.QueryRow(ctx, `SELECT data_type, column_default
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'audit_events' AND column_name = 'id'`).
		Scan(&dataType, &columnDefault)
	if err != nil {
		t.Fatalf("read audit_events.id: %v", err)
	}
	if dataType != "uuid" {
		t.Errorf("audit_events.id type = %q, want uuid", dataType)
	}
	if columnDefault == nil || !strings.Contains(*columnDefault, "gen_random_uuid()") {
		t.Errorf("audit_events.id default = %v, want gen_random_uuid()", columnDefault)
	}
}

// TestMigrations_ApplyToAnEmptySchemaOnce covers M1: every identity migration
// applies to an empty schema, and a second run applies nothing.
func TestMigrations_ApplyToAnEmptySchemaOnce(t *testing.T) {
	ctx := testContext(t)
	pool := newSchemaPool(ctx, t)

	migrateAll(ctx, t, pool)
	want := embeddedMigrations(t)
	if got := appliedMigrations(ctx, t, pool); !slices.Equal(got, want) {
		t.Fatalf("applied = %v, want %v", got, want)
	}
	for _, table := range identityTables {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatalf("look up %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s does not exist", table)
		}
	}
	requireAuditEventIDIsUUID(ctx, t, pool)

	migrateAll(ctx, t, pool)
	if got := appliedMigrations(ctx, t, pool); !slices.Equal(got, want) {
		t.Errorf("after a second run applied = %v, want %v", got, want)
	}
}

// TestMigrations_FromPhase07BackfillExistingRows covers M2 and M3: on a phase
// 07 schema with a merchant and its keys, MigrateEmbedded applies only 0006 to
// 0009, existing keys become secret keys named legacy with their hash intact,
// the merchant becomes sales-led and unapproved, and the phase 07 insert path
// still works.
func TestMigrations_FromPhase07BackfillExistingRows(t *testing.T) {
	ctx := testContext(t)
	pool := newSchemaPool(ctx, t)
	repo := pgstore.New(pool)

	migrateToPhase07(ctx, t, pool)
	merchantID := seedMerchant(ctx, t, repo)
	legacyKeys := []uuid.UUID{
		seedAPIKey(ctx, t, repo, merchantID, fixtureKeyPrefix+"active", domain.APIKeyActive),
		seedAPIKey(ctx, t, repo, merchantID, fixtureKeyPrefix+"revoked", domain.APIKeyRevoked),
	}

	migrateAll(ctx, t, pool)
	want := embeddedMigrations(t)
	if got := appliedMigrations(ctx, t, pool); !slices.Equal(got, want) {
		t.Fatalf("applied = %v, want %v", got, want)
	}

	var onboarding string
	var approvedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT onboarding, approved_at FROM merchants WHERE id = $1`, merchantID).
		Scan(&onboarding, &approvedAt); err != nil {
		t.Fatalf("read merchant: %v", err)
	}
	if onboarding != "sales_led" || approvedAt != nil {
		t.Errorf("merchant onboarding = %q, approved_at = %v; want sales_led and NULL", onboarding, approvedAt)
	}

	afterMigration := seedAPIKey(ctx, t, repo, merchantID, fixtureKeyPrefix+"after", domain.APIKeyActive)
	for _, id := range append(legacyKeys, afterMigration) {
		var kind, name string
		var hash, createdBy *string
		var expiresAt *time.Time
		if err := pool.QueryRow(ctx,
			`SELECT kind, name, hash, created_by_actor, expires_at FROM api_keys WHERE id = $1`, id).
			Scan(&kind, &name, &hash, &createdBy, &expiresAt); err != nil {
			t.Fatalf("read api key %s: %v", id, err)
		}
		if kind != backfilledKind || name != backfilledName {
			t.Errorf("key %s: kind = %q, name = %q; want %q, %q", id, kind, name, backfilledKind, backfilledName)
		}
		if hash == nil || *hash != fixtureKeyHash {
			t.Errorf("key %s: hash = %v, want it unchanged", id, hash)
		}
		if createdBy != nil || expiresAt != nil {
			t.Errorf("key %s: created_by_actor = %v, expires_at = %v; want NULL", id, createdBy, expiresAt)
		}
	}

	migrateAll(ctx, t, pool)
	if got := appliedMigrations(ctx, t, pool); !slices.Equal(got, want) {
		t.Errorf("after a second run applied = %v, want %v", got, want)
	}
}

// TestMigrations_RejectASecondActiveOwner covers M4: a merchant has at most
// one active owner, and a removed owner does not count.
func TestMigrations_RejectASecondActiveOwner(t *testing.T) {
	ctx := testContext(t)
	pool := newSchemaPool(ctx, t)
	migrateAll(ctx, t, pool)
	merchantID := seedMerchant(ctx, t, pgstore.New(pool))
	owner := seedUser(ctx, t, pool, fixtureOwnerEmail)
	second := seedUser(ctx, t, pool, fixtureSecondEmail)

	const insertOwner = `INSERT INTO merchant_memberships (merchant_id, user_id, role) VALUES ($1, $2, 'owner')`
	if _, err := pool.Exec(ctx, insertOwner, merchantID, owner); err != nil {
		t.Fatalf("first owner: %v", err)
	}
	_, err := pool.Exec(ctx, insertOwner, merchantID, second)
	requireViolation(t, "second active owner", err, sqlStateUniqueViolation, oneOwnerIndex)

	if _, err := pool.Exec(ctx,
		`UPDATE merchant_memberships SET status = 'removed' WHERE merchant_id = $1 AND user_id = $2`,
		merchantID, owner); err != nil {
		t.Fatalf("remove first owner: %v", err)
	}
	if _, err := pool.Exec(ctx, insertOwner, merchantID, second); err != nil {
		t.Errorf("owner after the first was removed: %v", err)
	}
}

// TestMigrations_APIKeyHashMatchesKind checks that only publishable keys have
// no hash.
func TestMigrations_APIKeyHashMatchesKind(t *testing.T) {
	ctx := testContext(t)
	pool := newSchemaPool(ctx, t)
	migrateAll(ctx, t, pool)
	merchantID := seedMerchant(ctx, t, pgstore.New(pool))
	hash := fixtureKeyHash

	tests := []struct {
		kind      string
		hash      *string
		wantError bool
	}{
		{kind: "secret", hash: &hash},
		{kind: "restricted", hash: &hash},
		{kind: "publishable", hash: nil},
		{kind: "secret", hash: nil, wantError: true},
		{kind: "restricted", hash: nil, wantError: true},
		{kind: "publishable", hash: &hash, wantError: true},
	}
	for i, tt := range tests {
		name := fmt.Sprintf("%s_with_hash_%t", tt.kind, tt.hash != nil)
		t.Run(name, func(t *testing.T) {
			_, err := pool.Exec(ctx,
				`INSERT INTO api_keys (merchant_id, prefix, hash, kind, name) VALUES ($1, $2, $3, $4, $5)`,
				merchantID, fmt.Sprintf("%s%d", fixtureKeyPrefix, i), tt.hash, tt.kind, name)
			if tt.wantError {
				requireViolation(t, name, err, sqlStateCheckViolation, hashMatchesKindCheck)
				return
			}
			if err != nil {
				t.Errorf("insert: %v", err)
			}
		})
	}
}
