//go:build integration

package pgstore_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	identitypg "github.com/iris-gate/iris-gate/libs/identity/infra/pgstore"
	"github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/domain"
	"github.com/iris-gate/iris-gate/libs/payments/infra/acquirer"
	"github.com/iris-gate/iris-gate/libs/payments/infra/pgstore"
	"github.com/iris-gate/iris-gate/libs/payments/testfixtures"
	"github.com/iris-gate/iris-gate/libs/shared/pgtenant"
)

// appURLEnv names the variable holding the iris_app connection URL, the role
// row-level security applies to (it has no BYPASSRLS).
const appURLEnv = "IRIS_APP_DATABASE_URL"

// Statements the row-level security tests run as iris_app.
const (
	selectAllIntentsSQL       = `SELECT count(*) FROM payment_intents`
	selectIntentByIDSQL       = `SELECT count(*) FROM payment_intents WHERE id = $1`
	updateAnyIntentSQL        = `UPDATE payment_intents SET description = $1 WHERE id = $2`
	reassignIntentSQL         = `UPDATE payment_intents SET merchant_id = $1 WHERE id = $2`
	insertIntentSQL           = `INSERT INTO payment_intents (id, merchant_id, amount_cents, currency, status, card_token, created_at) VALUES ($1,$2,1,'BRL','pending',$3,now())`
	selectAllIdempotencySQL   = `SELECT count(*) FROM idempotency_records`
	selectIdempotencyByKeySQL = `SELECT count(*) FROM idempotency_records WHERE merchant_id = $1 AND scope = $2 AND key = $3`
	insertIdempotencySQL      = `INSERT INTO idempotency_records (merchant_id, scope, key, request_hash, response_status, response_body) VALUES ($1,$2,$3,$4,$5,$6)`
	deleteIdempotencySQL      = `DELETE FROM idempotency_records WHERE merchant_id = $1`
	rowSecuritySQL            = `SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid = to_regclass($1)`
	foreignUpdateMarker       = "tampered"
)

// insufficientPrivilegeCode is the SQLSTATE Postgres raises when a row
// violates a row-level security WITH CHECK clause.
const insufficientPrivilegeCode = "42501"

// merchantScopedTables are the payments tables migration 0005 puts under
// forced row-level security.
var merchantScopedTables = []string{"payment_intents", "idempotency_records"}

// Fake idempotency record values shared by the tests.
const (
	idempotencyScope          = "create_payment_intent"
	idempotencyResponseStatus = http.StatusCreated
)

var (
	idempotencyRequestHash  = []byte("request-hash")
	idempotencyResponseBody = []byte(`{}`)
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

	exists, err := store.Save(ctx, a, idempotencyScope, key, []byte("ha"), idempotencyResponseStatus, []byte(`{"a":1}`))
	if err != nil || exists {
		t.Fatalf("A save: exists=%v err=%v", exists, err)
	}
	if exists, err = store.Save(ctx, b, idempotencyScope, key, []byte("hb"), idempotencyResponseStatus, []byte(`{"b":1}`)); err != nil || exists {
		t.Fatalf("B save with the same key: exists=%v err=%v", exists, err)
	}
	if exists, err = store.Save(ctx, a, "other_scope", key, []byte("hc"), idempotencyResponseStatus, idempotencyResponseBody); err != nil || exists {
		t.Fatalf("A save in another scope: exists=%v err=%v", exists, err)
	}
	if exists, err = store.Save(ctx, a, idempotencyScope, key, []byte("hz"), idempotencyResponseStatus, idempotencyResponseBody); err != nil || !exists {
		t.Fatalf("A duplicate save: exists=%v err=%v, want alreadyExists", exists, err)
	}

	recA, _ := store.Existing(ctx, a, idempotencyScope, key)
	recB, _ := store.Existing(ctx, b, idempotencyScope, key)
	if recA == nil || string(recA.RequestHash) != "ha" || recB == nil || string(recB.RequestHash) != "hb" {
		t.Fatalf("records mixed up: A=%+v B=%+v", recA, recB)
	}
	if rec, _ := store.Existing(ctx, uuid.New(), idempotencyScope, key); rec != nil {
		t.Error("unknown merchant saw a record")
	}
}

// ---------- D3, D4: persisted declines and optimistic locking ----------

// racingCallers is how many goroutines race on one intent.
const racingCallers = 8

func seedPendingIntent(t *testing.T, repo *pgstore.Repo, merchantID uuid.UUID, cardToken string) domain.PaymentIntent {
	t.Helper()
	amount, _ := domain.NewMoney(1000, "BRL")
	pi, err := domain.NewPaymentIntent(uuid.New(), merchantID, amount, cardToken, "", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateWithOutbox(context.Background(), pi, application.OutboxMessage{Topic: "t", Key: "k", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	return pi
}

// outboxRows counts the outbox rows of one topic for one intent.
func outboxRows(t *testing.T, pool *pgxpool.Pool, topic string, intentID uuid.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox_messages WHERE topic = $1 AND payload->>'payment_intent_id' = $2`,
		topic, intentID.String()).Scan(&n)
	if err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return n
}

// raceAuthorize runs racingCallers authorizations of one intent at once and
// returns their errors.
func raceAuthorize(uc *application.AuthorizePaymentUseCase, merchantID, id uuid.UUID) []error {
	errs := make([]error, racingCallers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range racingCallers {
		wg.Go(func() {
			<-start
			_, errs[i] = uc.Execute(context.Background(), merchantID, id)
		})
	}
	close(start)
	wg.Wait()
	return errs
}

// AC4, I6: a decline persists failed, failure_code and one failed.v1 row.
func TestIntegration_Decline_PersistsFailedWithOutboxRow(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	repo := pgstore.New(pool)
	merchant := seedMerchant(t, pool)
	pi := seedPendingIntent(t, repo, merchant, testfixtures.TokenDeclined)
	uc := application.NewAuthorizePaymentUseCase(repo, acquirer.NewFake(testfixtures.TokenDeclined), application.SystemClock{}, application.UUIDGen{})

	if _, err := uc.Execute(ctx, merchant, pi.ID); !errors.Is(err, domain.ErrAcquirerRejected) {
		t.Fatalf("err = %v, want ErrAcquirerRejected", err)
	}

	if n := outboxRows(t, pool, application.TopicPaymentIntentFailedV1, pi.ID); n != 1 {
		t.Errorf("failed outbox rows = %d, want 1", n)
	}
	// Read through the merchant-scoped repository: payment_intents has forced
	// RLS, so an unscoped read sees the row only as a superuser.
	got, err := repo.GetByID(ctx, merchant, pi.ID)
	if err != nil || got.Status != domain.PaymentIntentFailed || got.FailureCode != domain.FailureCardDeclined || got.Version != 1 {
		t.Errorf("GetByID = (%+v, %v), want failed, card_declined, version 1", got, err)
	}
}

// AC5, I1, I5: N concurrent authorizations → one acquirer authorization, one
// authorized row; every loser gets ErrConcurrentUpdate (409) or the idempotent
// result, and writes nothing.
func TestIntegration_ConcurrentAuthorize_OneAuthorization(t *testing.T) {
	pool := newPool(t)
	repo := pgstore.New(pool)
	merchant := seedMerchant(t, pool)
	pi := seedPendingIntent(t, repo, merchant, testfixtures.TokenApproved)
	fake := acquirer.NewFake()
	uc := application.NewAuthorizePaymentUseCase(repo, fake, application.SystemClock{}, application.UUIDGen{})

	succeeded := 0
	for _, err := range raceAuthorize(uc, merchant, pi.ID) {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, domain.ErrConcurrentUpdate):
		default:
			t.Errorf("unexpected err: %v", err)
		}
	}
	if succeeded == 0 {
		t.Error("no caller succeeded")
	}
	if n := fake.Authorizations(); n != 1 {
		t.Errorf("acquirer authorizations = %d, want 1", n)
	}
	if n := outboxRows(t, pool, application.TopicPaymentIntentAuthorizedV1, pi.ID); n != 1 {
		t.Errorf("authorized outbox rows = %d, want 1", n)
	}
	got, err := repo.GetByID(context.Background(), merchant, pi.ID)
	if err != nil || got.Status != domain.PaymentIntentAuthorized || got.Version != 1 {
		t.Errorf("intent = (%s v%d, %v), want authorized v1", got.Status, got.Version, err)
	}
}

// I3: concurrent authorizations of a declined card all see the one decline.
func TestIntegration_ConcurrentAuthorize_Declined_OneDecision(t *testing.T) {
	pool := newPool(t)
	repo := pgstore.New(pool)
	merchant := seedMerchant(t, pool)
	pi := seedPendingIntent(t, repo, merchant, testfixtures.TokenDeclined)
	fake := acquirer.NewFake(testfixtures.TokenDeclined)
	uc := application.NewAuthorizePaymentUseCase(repo, fake, application.SystemClock{}, application.UUIDGen{})

	for _, err := range raceAuthorize(uc, merchant, pi.ID) {
		if !errors.Is(err, domain.ErrAcquirerRejected) && !errors.Is(err, domain.ErrConcurrentUpdate) && !errors.Is(err, domain.ErrInvalidStatusTransition) {
			t.Errorf("err = %v, want a decline, a lost race or a transition error", err)
		}
	}
	if n := fake.Authorizations(); n != 1 {
		t.Errorf("acquirer authorizations = %d, want 1", n)
	}
	if n := outboxRows(t, pool, application.TopicPaymentIntentFailedV1, pi.ID); n != 1 {
		t.Errorf("failed outbox rows = %d, want 1", n)
	}
	if n := outboxRows(t, pool, application.TopicPaymentIntentAuthorizedV1, pi.ID); n != 0 {
		t.Errorf("authorized outbox rows = %d, want 0", n)
	}
}

// I2: authorize racing capture never writes two events of one kind and ends
// in a valid state.
func TestIntegration_AuthorizeRacingCapture_ValidFinalState(t *testing.T) {
	pool := newPool(t)
	repo := pgstore.New(pool)
	merchant := seedMerchant(t, pool)
	pi := seedPendingIntent(t, repo, merchant, testfixtures.TokenApproved)
	authorize := application.NewAuthorizePaymentUseCase(repo, acquirer.NewFake(), application.SystemClock{}, application.UUIDGen{})
	capture := application.NewCapturePaymentUseCase(repo, application.SystemClock{}, application.UUIDGen{})

	start := make(chan struct{})
	var wg sync.WaitGroup
	for range racingCallers {
		wg.Go(func() {
			<-start
			_, _ = authorize.Execute(context.Background(), merchant, pi.ID)
		})
		wg.Go(func() {
			<-start
			_, _ = capture.Execute(context.Background(), merchant, pi.ID)
		})
	}
	close(start)
	wg.Wait()

	got, err := repo.GetByID(context.Background(), merchant, pi.ID)
	if err != nil {
		t.Fatal(err)
	}
	authorized := outboxRows(t, pool, application.TopicPaymentIntentAuthorizedV1, pi.ID)
	captured := outboxRows(t, pool, application.TopicPaymentIntentCapturedV1, pi.ID)
	if authorized != 1 || captured > 1 {
		t.Errorf("events: authorized = %d captured = %d, want 1 and at most 1", authorized, captured)
	}
	switch got.Status {
	case domain.PaymentIntentAuthorized:
		if captured != 0 {
			t.Errorf("authorized intent has %d captured events", captured)
		}
	case domain.PaymentIntentCaptured:
		if captured != 1 {
			t.Errorf("captured intent has %d captured events", captured)
		}
	default:
		t.Errorf("final status = %s, want authorized or captured", got.Status)
	}
	if want := authorized + captured; got.Version != want {
		t.Errorf("version = %d, want %d (one per update)", got.Version, want)
	}
}

// R1: a stale version writes nothing and is ErrConcurrentUpdate; a foreign
// merchant is ErrPaymentIntentNotFound even with the right version.
func TestIntegration_UpdateWithOutbox_StaleVersion(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	repo := pgstore.New(pool)
	owner, other := seedMerchant(t, pool), seedMerchant(t, pool)
	pi := seedPendingIntent(t, repo, owner, testfixtures.TokenApproved)
	msg := func() application.OutboxMessage {
		return application.OutboxMessage{Topic: application.TopicPaymentIntentAuthorizedV1, Payload: []byte(`{"payment_intent_id":"` + pi.ID.String() + `"}`)}
	}

	fresh := pi
	if err := fresh.MarkAuthorized(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateWithOutbox(ctx, fresh, msg()); err != nil {
		t.Fatalf("first update: %v", err)
	}
	if err := repo.UpdateWithOutbox(ctx, fresh, msg()); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Errorf("stale update err = %v, want ErrConcurrentUpdate", err)
	}
	foreign := fresh
	foreign.MerchantID = other
	foreign.Version = 1
	if err := repo.UpdateWithOutbox(ctx, foreign, msg()); !errors.Is(err, domain.ErrPaymentIntentNotFound) {
		t.Errorf("foreign update err = %v, want ErrPaymentIntentNotFound", err)
	}
	if n := outboxRows(t, pool, application.TopicPaymentIntentAuthorizedV1, pi.ID); n != 1 {
		t.Errorf("outbox rows = %d, want 1 (rejected updates write nothing)", n)
	}
}

// M1b: 0004 on rows in the old shape gives version 0 and a NULL failure code,
// and the check rejects a failure code on a non-failed intent.
func TestIntegration_Migration0004_OnLegacyRows(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	merchant := seedMerchant(t, pool)
	body, err := os.ReadFile("migrations/0004_payment_intent_version_and_failure.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // leave the shared schema untouched

	legacyID := uuid.New()
	for _, stmt := range []string{
		`ALTER TABLE payment_intents DROP CONSTRAINT payment_intents_failure_code_only_when_failed`,
		`ALTER TABLE payment_intents DROP COLUMN version, DROP COLUMN failure_code`,
	} {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO payment_intents (id, merchant_id, amount_cents, currency, card_token) VALUES ($1,$2,100,'BRL',$3)`,
		legacyID, merchant, testfixtures.TokenApproved); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if _, err := tx.Exec(ctx, string(body)); err != nil {
		t.Fatalf("apply 0004: %v", err)
	}

	var version int
	var failureCode *string
	if err := tx.QueryRow(ctx, `SELECT version, failure_code FROM payment_intents WHERE id = $1`, legacyID).
		Scan(&version, &failureCode); err != nil {
		t.Fatal(err)
	}
	if version != 0 || failureCode != nil {
		t.Errorf("legacy row = (v%d, %v), want (v0, NULL)", version, failureCode)
	}

	_, err = tx.Exec(ctx, `UPDATE payment_intents SET failure_code = 'card_declined' WHERE id = $1`, legacyID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "payment_intents_failure_code_only_when_failed" {
		t.Errorf("failure code on a pending intent: err = %v, want the check violation", err)
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

// ---------- Phase 07: row-level security on payments tables ----------
//
// The TestIntegration_RLS_* tests need the iris_app role, so they fail (never
// skip) when IRIS_APP_DATABASE_URL is unset. CI runs them in the api-int job,
// which provisions the roles; a job without roles opts out explicitly with
// `make test-int SKIP_RLS=1`.

// newAppPool connects as iris_app. The test fails when IRIS_APP_DATABASE_URL
// is unset: a skip here would hide that row-level security was never checked.
func newAppPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv(appURLEnv)
	if url == "" {
		t.Fatal(appURLEnv + " not set: the RLS tests need the iris_app role (use SKIP_RLS=1 where the roles do not exist)")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect as iris_app: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedIdempotencyRecord stores one idempotency record for merchantID through
// pool and returns its key.
func seedIdempotencyRecord(t *testing.T, pool *pgxpool.Pool, merchantID uuid.UUID) string {
	t.Helper()
	key := uuid.NewString()
	exists, err := pgstore.NewIdempotencyStore(pool).Save(context.Background(), merchantID, idempotencyScope, key,
		idempotencyRequestHash, idempotencyResponseStatus, idempotencyResponseBody)
	if err != nil || exists {
		t.Fatalf("seed idempotency record: exists=%v err=%v", exists, err)
	}
	return key
}

// countAsMerchant runs a count(*) query on pool inside WithMerchant(merchantID).
func countAsMerchant(t *testing.T, pool *pgxpool.Pool, merchantID uuid.UUID, query string, args ...any) int {
	t.Helper()
	var n int
	err := pgtenant.WithMerchant(context.Background(), pool, merchantID.String(), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, query, args...).Scan(&n)
	})
	if err != nil {
		t.Fatalf("count as merchant %s: %v", merchantID, err)
	}
	return n
}

// execAsMerchant runs one statement on pool inside WithMerchant(merchantID).
func execAsMerchant(pool *pgxpool.Pool, merchantID uuid.UUID, query string, args ...any) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	err := pgtenant.WithMerchant(context.Background(), pool, merchantID.String(), func(ctx context.Context, tx pgx.Tx) error {
		var execErr error
		tag, execErr = tx.Exec(ctx, query, args...)
		return execErr
	})
	return tag, err
}

// requireOwnRowVisible is the positive control of an isolation test: inside
// WithMerchant(owner), the iris_app pool sees exactly one row of the owner's
// seeded data. Without it, a zero count for another merchant could pass only
// because DATABASE_URL and IRIS_APP_DATABASE_URL point at different databases.
func requireOwnRowVisible(t *testing.T, appPool *pgxpool.Pool, owner uuid.UUID, query string, args ...any) {
	t.Helper()
	if n := countAsMerchant(t, appPool, owner, query, args...); n != 1 {
		t.Fatalf("owner sees %d of its own rows as iris_app, want 1 (do both URLs point at the same database?)", n)
	}
}

// requireRLSViolation fails unless err is the row-level security violation
// Postgres raises for a WITH CHECK failure.
func requireRLSViolation(t *testing.T, err error, action string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != insufficientPrivilegeCode {
		t.Errorf("%s: err = %v, want SQLSTATE %s (row-level security violation)", action, err, insufficientPrivilegeCode)
	}
}

// intentEvent is an outbox message whose payload names the intent, so
// outboxRows can count it.
func intentEvent(topic string, id uuid.UUID) application.OutboxMessage {
	return application.OutboxMessage{Topic: topic, Key: id.String(), Payload: fmt.Appendf(nil, `{"payment_intent_id":%q}`, id.String())}
}

func seedIntentFor(t *testing.T, repo *pgstore.Repo, merchantID uuid.UUID) domain.PaymentIntent {
	t.Helper()
	return seedPendingIntent(t, repo, merchantID, testfixtures.TokenApproved)
}

// TestIntegration_RLS_NoMerchantSeesNoRows covers spec 07 AC3: without
// app.merchant_id, iris_app reads zero rows of either table even though rows
// exist.
func TestIntegration_RLS_NoMerchantSeesNoRows(t *testing.T) {
	pool := newPool(t)
	appPool := newAppPool(t)
	ctx := context.Background()
	merchant := seedMerchant(t, pool)
	pi := seedIntentFor(t, pgstore.New(pool), merchant)
	key := seedIdempotencyRecord(t, pool, merchant)

	requireOwnRowVisible(t, appPool, merchant, selectIntentByIDSQL, pi.ID)
	requireOwnRowVisible(t, appPool, merchant, selectIdempotencyByKeySQL, merchant, idempotencyScope, key)

	// Twice per table, so a statement may reuse a connection whose
	// transaction-local setting already ended (empty string, not NULL).
	for _, query := range []string{selectAllIntentsSQL, selectAllIdempotencySQL} {
		for range 2 {
			var n int
			if err := appPool.QueryRow(ctx, query).Scan(&n); err != nil {
				t.Fatalf("%s without merchant: %v", query, err)
			}
			if n != 0 {
				t.Errorf("%s without merchant = %d, want 0", query, n)
			}
		}
	}
}

// TestIntegration_RLS_OtherMerchantInvisibleAndImmutable covers spec 07 AC4:
// with app.merchant_id = A, merchant B's rows are invisible and cannot be
// updated or inserted.
func TestIntegration_RLS_OtherMerchantInvisibleAndImmutable(t *testing.T) {
	pool := newPool(t)
	appPool := newAppPool(t)
	ctx := context.Background()
	repo := pgstore.New(pool)
	a, b := seedMerchant(t, pool), seedMerchant(t, pool)
	intentA := seedIntentFor(t, repo, a)
	intentB := seedIntentFor(t, repo, b)

	requireOwnRowVisible(t, appPool, a, selectIntentByIDSQL, intentA.ID)
	if n := countAsMerchant(t, appPool, a, selectAllIntentsSQL); n != 1 {
		t.Errorf("A sees %d intents, want 1 (its own; B's is hidden)", n)
	}
	if n := countAsMerchant(t, appPool, a, selectIntentByIDSQL, intentB.ID); n != 0 {
		t.Errorf("A sees %d of B's intents, want 0", n)
	}

	tag, err := execAsMerchant(appPool, a, updateAnyIntentSQL, foreignUpdateMarker, intentB.ID)
	if err != nil {
		t.Fatalf("A updates B's intent: %v", err)
	}
	if tag.RowsAffected() != 0 {
		t.Errorf("A updated %d of B's rows, want 0", tag.RowsAffected())
	}

	_, err = execAsMerchant(appPool, a, insertIntentSQL, uuid.New(), b, testfixtures.TokenApproved)
	requireRLSViolation(t, err, "A inserts an intent for B")

	got, err := repo.GetByID(ctx, b, intentB.ID)
	if err != nil || got.Description != intentB.Description {
		t.Errorf("B's intent changed: %+v err=%v", got, err)
	}
}

// TestIntegration_RLS_OwnRowCannotMoveToAnotherMerchant checks WITH CHECK on
// UPDATE: merchant A cannot reassign its own intent to merchant B.
func TestIntegration_RLS_OwnRowCannotMoveToAnotherMerchant(t *testing.T) {
	pool := newPool(t)
	appPool := newAppPool(t)
	ctx := context.Background()
	repo := pgstore.New(pool)
	a, b := seedMerchant(t, pool), seedMerchant(t, pool)
	intentA := seedIntentFor(t, repo, a)

	requireOwnRowVisible(t, appPool, a, selectIntentByIDSQL, intentA.ID)
	_, err := execAsMerchant(appPool, a, reassignIntentSQL, b, intentA.ID)
	requireRLSViolation(t, err, "A moves its intent to B")

	got, err := repo.GetByID(ctx, a, intentA.ID)
	if err != nil || got.MerchantID != a {
		t.Errorf("A's intent after the move attempt = %+v err=%v, want it still owned by A", got, err)
	}
}

// TestIntegration_RLS_IdempotencyRecordsIsolated checks the policy on
// idempotency_records with raw SQL, independent of the repository's own
// merchant_id filter: B's records are invisible to A, and A cannot insert or
// delete records of B.
func TestIntegration_RLS_IdempotencyRecordsIsolated(t *testing.T) {
	pool := newPool(t)
	appPool := newAppPool(t)
	ctx := context.Background()
	a, b := seedMerchant(t, pool), seedMerchant(t, pool)
	keyA := seedIdempotencyRecord(t, pool, a)
	keyB := seedIdempotencyRecord(t, pool, b)

	requireOwnRowVisible(t, appPool, a, selectIdempotencyByKeySQL, a, idempotencyScope, keyA)
	if n := countAsMerchant(t, appPool, a, selectAllIdempotencySQL); n != 1 {
		t.Errorf("A sees %d idempotency records, want 1 (its own; B's is hidden)", n)
	}
	if n := countAsMerchant(t, appPool, a, selectIdempotencyByKeySQL, b, idempotencyScope, keyB); n != 0 {
		t.Errorf("A sees %d of B's idempotency records, want 0", n)
	}

	_, err := execAsMerchant(appPool, a, insertIdempotencySQL, b, idempotencyScope, uuid.NewString(),
		idempotencyRequestHash, idempotencyResponseStatus, idempotencyResponseBody)
	requireRLSViolation(t, err, "A inserts an idempotency record for B")

	tag, err := execAsMerchant(appPool, a, deleteIdempotencySQL, b)
	if err != nil {
		t.Fatalf("A deletes B's idempotency records: %v", err)
	}
	if tag.RowsAffected() != 0 {
		t.Errorf("A deleted %d of B's idempotency records, want 0", tag.RowsAffected())
	}
	if rec, err := pgstore.NewIdempotencyStore(pool).Existing(ctx, b, idempotencyScope, keyB); err != nil || rec == nil {
		t.Errorf("B's idempotency record = %+v err=%v, want it unchanged", rec, err)
	}
}

// TestIntegration_RLS_EnabledAndForcedOnMerchantTables reads the live catalog:
// row-level security is enabled and forced (so it binds the table owner too)
// on every merchant-scoped payments table.
func TestIntegration_RLS_EnabledAndForcedOnMerchantTables(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	for _, table := range merchantScopedTables {
		var enabled, forced bool
		if err := pool.QueryRow(ctx, rowSecuritySQL, table).Scan(&enabled, &forced); err != nil {
			t.Fatalf("%s: read pg_class: %v", table, err)
		}
		if !enabled || !forced {
			t.Errorf("%s: relrowsecurity = %v, relforcerowsecurity = %v, want both true", table, enabled, forced)
		}
	}
}

// TestIntegration_RLS_UpdateWithOutboxAsIrisApp runs UpdateWithOutbox on the
// iris_app pool: another merchant gets ErrPaymentIntentNotFound and changes
// nothing; the owner's update and its outbox row go through.
func TestIntegration_RLS_UpdateWithOutboxAsIrisApp(t *testing.T) {
	pool := newPool(t)
	appPool := newAppPool(t)
	ctx := context.Background()
	repo := pgstore.New(appPool)
	owner, other := seedMerchant(t, pool), seedMerchant(t, pool)
	pi := seedIntentFor(t, repo, owner)
	requireOwnRowVisible(t, appPool, owner, selectIntentByIDSQL, pi.ID)

	authorized := pi
	if err := authorized.MarkAuthorized(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	event := intentEvent(application.TopicPaymentIntentAuthorizedV1, pi.ID)

	foreign := authorized
	foreign.MerchantID = other
	if err := repo.UpdateWithOutbox(ctx, foreign, event); !errors.Is(err, domain.ErrPaymentIntentNotFound) {
		t.Errorf("foreign update err = %v, want ErrPaymentIntentNotFound", err)
	}
	got, err := repo.GetByID(ctx, owner, pi.ID)
	if err != nil || got.Status != domain.PaymentIntentPending || got.Version != pi.Version {
		t.Errorf("after the foreign update: (%s v%d, %v), want pending v%d", got.Status, got.Version, err, pi.Version)
	}
	if n := outboxRows(t, pool, application.TopicPaymentIntentAuthorizedV1, pi.ID); n != 0 {
		t.Errorf("authorized outbox rows after the foreign update = %d, want 0", n)
	}

	if err := repo.UpdateWithOutbox(ctx, authorized, event); err != nil {
		t.Fatalf("owner update: %v", err)
	}
	got, err = repo.GetByID(ctx, owner, pi.ID)
	if err != nil || got.Status != domain.PaymentIntentAuthorized || got.Version != pi.Version+1 {
		t.Errorf("after the owner update: (%s v%d, %v), want authorized v%d", got.Status, got.Version, err, pi.Version+1)
	}
	if n := outboxRows(t, pool, application.TopicPaymentIntentAuthorizedV1, pi.ID); n != 1 {
		t.Errorf("authorized outbox rows after the owner update = %d, want 1", n)
	}
}

// TestIntegration_RLS_RepositoriesWorkAsIrisApp runs the repositories on the
// iris_app pool: the owner reads and writes, another merchant cannot.
func TestIntegration_RLS_RepositoriesWorkAsIrisApp(t *testing.T) {
	pool := newPool(t)
	appPool := newAppPool(t)
	ctx := context.Background()
	repo := pgstore.New(appPool)
	owner, other := seedMerchant(t, pool), seedMerchant(t, pool)
	pi := seedIntentFor(t, repo, owner)

	if _, err := repo.GetByID(ctx, owner, pi.ID); err != nil {
		t.Errorf("owner read: %v", err)
	}
	if _, err := repo.GetByID(ctx, other, pi.ID); !errors.Is(err, domain.ErrPaymentIntentNotFound) {
		t.Errorf("foreign read err = %v, want ErrPaymentIntentNotFound", err)
	}
	list, err := repo.ListByMerchant(ctx, other, 10)
	if err != nil || len(list) != 0 {
		t.Errorf("foreign list = %d err=%v, want empty", len(list), err)
	}

	store := pgstore.NewIdempotencyStore(appPool)
	key := uuid.NewString()
	if exists, err := store.Save(ctx, owner, idempotencyScope, key, idempotencyRequestHash, idempotencyResponseStatus, idempotencyResponseBody); err != nil || exists {
		t.Fatalf("save: exists=%v err=%v", exists, err)
	}
	if rec, err := store.Existing(ctx, other, idempotencyScope, key); err != nil || rec != nil {
		t.Errorf("foreign idempotency read = %+v err=%v, want none", rec, err)
	}
}
