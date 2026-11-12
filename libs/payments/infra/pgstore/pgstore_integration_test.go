//go:build integration

package pgstore_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	identitypg "github.com/iris-gate/iris-gate/libs/identity/infra/pgstore"
	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/domain"
	"github.com/iris-gate/iris-gate/libs/payments/infra/pgstore"
	"github.com/iris-gate/iris-gate/libs/payments/testfixtures"
)

// newPool connects to DATABASE_URL, applies identity then payments migrations
// and returns the pool. The test skips when DATABASE_URL is unset.
func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := identitypg.MigrateEmbedded(ctx, pool); err != nil {
		t.Fatalf("identity migrate: %v", err)
	}
	if err := pgstore.MigrateEmbedded(ctx, pool); err != nil {
		t.Fatalf("payments migrate: %v", err)
	}
	return pool
}

func seedMerchant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	doc := fmt.Sprintf("%014d", uint64(id.ID()))
	_, err := pool.Exec(context.Background(),
		`INSERT INTO merchants (id, legal_name, document, contact_email) VALUES ($1,'Test',$2,'t@example.com')`,
		id, doc)
	if err != nil {
		t.Fatalf("seed merchant: %v", err)
	}
	return id
}

func TestIntegration_GetByID_IsScopedToMerchant(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	repo := pgstore.New(pool)
	owner, other := seedMerchant(t, pool), seedMerchant(t, pool)

	amount, _ := domain.NewMoney(1000, "BRL")
	pi, err := domain.NewPaymentIntent(uuid.New(), owner, amount, testfixtures.TokenApproved, "", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateWithOutbox(ctx, pi, application.OutboxMessage{Topic: "t", Key: "k", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.GetByID(ctx, owner, pi.ID); err != nil {
		t.Errorf("owner read: %v", err)
	}
	if _, err := repo.GetByID(ctx, other, pi.ID); err != domain.ErrPaymentIntentNotFound {
		t.Errorf("foreign read err = %v, want ErrPaymentIntentNotFound", err)
	}

	pi.Status = domain.PaymentIntentAuthorized
	foreign := pi
	foreign.MerchantID = other
	if err := repo.UpdateWithOutbox(ctx, foreign, application.OutboxMessage{Topic: "t", Payload: []byte(`{}`)}); err != domain.ErrPaymentIntentNotFound {
		t.Errorf("foreign update err = %v, want ErrPaymentIntentNotFound", err)
	}
	got, _ := repo.GetByID(ctx, owner, pi.ID)
	if got.Status != domain.PaymentIntentPending {
		t.Errorf("status = %v, want pending", got.Status)
	}
}

func TestIntegration_IdempotencyKeyIsPerMerchantAndScope(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	store := pgstore.NewIdempotencyStore(pool)
	a, b := seedMerchant(t, pool), seedMerchant(t, pool)
	key := uuid.NewString()

	exists, err := store.Save(ctx, a, "create_payment_intent", key, []byte("ha"), 201, []byte(`{"a":1}`))
	if err != nil || exists {
		t.Fatalf("A save: exists=%v err=%v", exists, err)
	}
	if exists, err = store.Save(ctx, b, "create_payment_intent", key, []byte("hb"), 201, []byte(`{"b":1}`)); err != nil || exists {
		t.Fatalf("B save with the same key: exists=%v err=%v", exists, err)
	}
	if exists, err = store.Save(ctx, a, "other_scope", key, []byte("hc"), 201, []byte(`{}`)); err != nil || exists {
		t.Fatalf("A save in another scope: exists=%v err=%v", exists, err)
	}
	if exists, err = store.Save(ctx, a, "create_payment_intent", key, []byte("hz"), 201, []byte(`{}`)); err != nil || !exists {
		t.Fatalf("A duplicate save: exists=%v err=%v, want alreadyExists", exists, err)
	}

	recA, _ := store.Existing(ctx, a, "create_payment_intent", key)
	recB, _ := store.Existing(ctx, b, "create_payment_intent", key)
	if recA == nil || string(recA.RequestHash) != "ha" || recB == nil || string(recB.RequestHash) != "hb" {
		t.Fatalf("records mixed up: A=%+v B=%+v", recA, recB)
	}
	if rec, _ := store.Existing(ctx, uuid.New(), "create_payment_intent", key); rec != nil {
		t.Error("unknown merchant saw a record")
	}
}

// TestIntegration_Migration0003_DeletesLegacyRows replays 0003 on a table that
// holds a row in the old shape and checks the key afterwards.
func TestIntegration_Migration0003_DeletesLegacyRows(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	body, err := os.ReadFile("migrations/0003_idempotency_per_merchant.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // leave the shared schema untouched

	for _, stmt := range []string{
		`DROP TABLE idempotency_records`,
		`CREATE TABLE idempotency_records (scope TEXT NOT NULL, key TEXT NOT NULL, request_hash BYTEA NOT NULL,
			response_status INT NOT NULL, response_body BYTEA NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (scope, key))`,
		`INSERT INTO idempotency_records VALUES ('create_payment_intent','legacy','\x00',201,'\x00')`,
	} {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := tx.Exec(ctx, string(body)); err != nil {
		t.Fatalf("apply 0003: %v", err)
	}

	var rows int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM idempotency_records`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("rows after migration = %d err=%v, want 0", rows, err)
	}
	var pk string
	err = tx.QueryRow(ctx, `
		SELECT string_agg(a.attname, ',' ORDER BY array_position(i.indkey::int2[], a.attnum))
		FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = 'idempotency_records'::regclass AND i.indisprimary`).Scan(&pk)
	if err != nil || pk != "merchant_id,scope,key" {
		t.Fatalf("primary key = %q err=%v, want merchant_id,scope,key", pk, err)
	}
}
