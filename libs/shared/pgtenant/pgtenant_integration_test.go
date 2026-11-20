//go:build integration

package pgtenant_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iris-gate/iris-gate/libs/shared/pgtenant"
)

// integrationTimeout bounds each test, so a connection the helper failed to
// release fails the test on the one-connection pool instead of hanging it.
const integrationTimeout = 10 * time.Second

// sessionSQL reads the merchant setting (an empty string once a
// transaction-local value has ended, NULL if it was never set; both read as
// empty) and the backend pid, which proves two reads ran on the same physical
// connection.
const sessionSQL = "SELECT coalesce(current_setting('app.merchant_id', true), ''), pg_backend_pid()"

// session is what sessionSQL reads.
type session struct {
	merchantID string
	backendPID int32
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// newSingleConnPool connects to DATABASE_URL with MaxConns = 1, so every call
// reuses one physical connection. It skips the test when DATABASE_URL is
// unset. No table is needed: any Postgres role can run these tests.
func newSingleConnPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), integrationTimeout)
	t.Cleanup(cancel)
	return ctx
}

func readSession(ctx context.Context, t *testing.T, q rowQuerier) session {
	t.Helper()
	var s session
	if err := q.QueryRow(ctx, sessionSQL).Scan(&s.merchantID, &s.backendPID); err != nil {
		t.Fatalf("read session: %v", err)
	}
	return s
}

// withMerchantSession runs WithMerchant and returns what fn saw inside the
// transaction.
func withMerchantSession(ctx context.Context, t *testing.T, pool *pgxpool.Pool, merchantID string, fnErr error) (session, error) {
	t.Helper()
	var inside session
	err := pgtenant.WithMerchant(ctx, pool, merchantID, func(ctx context.Context, tx pgx.Tx) error {
		inside = readSession(ctx, t, tx)
		return fnErr
	})
	return inside, err
}

// assertMerchantCleared checks that the next use of the pool's single
// connection sees no merchant.
func assertMerchantCleared(ctx context.Context, t *testing.T, pool *pgxpool.Pool, inside session) {
	t.Helper()
	after := readSession(ctx, t, pool)
	if after.backendPID != inside.backendPID {
		t.Fatalf("backend pid changed from %d to %d: the leak check needs the same connection", inside.backendPID, after.backendPID)
	}
	if after.merchantID != "" {
		t.Errorf("app.merchant_id after the transaction = %q, want empty", after.merchantID)
	}
}

func TestIntegration_MerchantIsVisibleInsideAndGoneAfterCommit(t *testing.T) {
	pool := newSingleConnPool(t)
	ctx := testContext(t)

	inside, err := withMerchantSession(ctx, t, pool, merchantA, nil)
	if err != nil {
		t.Fatalf("WithMerchant: %v", err)
	}

	if inside.merchantID != merchantA {
		t.Errorf("app.merchant_id inside = %q, want %q", inside.merchantID, merchantA)
	}
	assertMerchantCleared(ctx, t, pool, inside)
}

func TestIntegration_MerchantIsGoneAfterRollback(t *testing.T) {
	pool := newSingleConnPool(t)
	ctx := testContext(t)

	inside, err := withMerchantSession(ctx, t, pool, merchantA, errFn)
	if !errors.Is(err, errFn) {
		t.Fatalf("err = %v, want %v", err, errFn)
	}

	if inside.merchantID != merchantA {
		t.Errorf("app.merchant_id inside = %q, want %q", inside.merchantID, merchantA)
	}
	assertMerchantCleared(ctx, t, pool, inside)
}

func TestIntegration_SequentialCallsDoNotLeakTheMerchant(t *testing.T) {
	pool := newSingleConnPool(t)
	ctx := testContext(t)

	first, err := withMerchantSession(ctx, t, pool, merchantA, nil)
	if err != nil {
		t.Fatalf("first WithMerchant: %v", err)
	}
	assertMerchantCleared(ctx, t, pool, first)
	second, err := withMerchantSession(ctx, t, pool, merchantB, nil)
	if err != nil {
		t.Fatalf("second WithMerchant: %v", err)
	}

	if second.backendPID != first.backendPID {
		t.Fatalf("backend pid changed from %d to %d: the calls must share the connection", first.backendPID, second.backendPID)
	}
	if second.merchantID != merchantB {
		t.Errorf("app.merchant_id in the second call = %q, want %q", second.merchantID, merchantB)
	}
	assertMerchantCleared(ctx, t, pool, second)
}

func TestIntegration_UpperCaseMerchantIsStoredInCanonicalForm(t *testing.T) {
	pool := newSingleConnPool(t)
	ctx := testContext(t)

	inside, err := withMerchantSession(ctx, t, pool, strings.ToUpper(merchantA), nil)
	if err != nil {
		t.Fatalf("WithMerchant: %v", err)
	}

	if inside.merchantID != merchantA {
		t.Errorf("app.merchant_id inside = %q, want %q", inside.merchantID, merchantA)
	}
}

func TestIntegration_PanicReleasesTheConnection(t *testing.T) {
	pool := newSingleConnPool(t)
	ctx := testContext(t)

	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("recovered %v, want the original panic value", got)
			}
		}()
		_ = pgtenant.WithMerchant(ctx, pool, merchantA, func(context.Context, pgx.Tx) error {
			panic(panicValue)
		})
	}()

	// With one connection, this call waits until integrationTimeout if the
	// panic left the connection checked out.
	inside, err := withMerchantSession(ctx, t, pool, merchantB, nil)
	if err != nil {
		t.Fatalf("WithMerchant after a panic: %v", err)
	}
	if inside.merchantID != merchantB {
		t.Errorf("app.merchant_id inside = %q, want %q", inside.merchantID, merchantB)
	}
	assertMerchantCleared(ctx, t, pool, inside)
}

func TestIntegration_ContextDoneDuringFnDoesNotCommitAndFreesThePool(t *testing.T) {
	pool := newSingleConnPool(t)
	ctx, cancel := context.WithCancel(testContext(t))
	defer cancel()

	err := pgtenant.WithMerchant(ctx, pool, merchantA, func(context.Context, pgx.Tx) error {
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want it to wrap context.Canceled", err)
	}

	// pgx closes a connection whose rollback saw a done context; the pool must
	// still serve the next call.
	next := testContext(t)
	inside, err := withMerchantSession(next, t, pool, merchantB, nil)
	if err != nil {
		t.Fatalf("WithMerchant after a cancelled call: %v", err)
	}
	if inside.merchantID != merchantB {
		t.Errorf("app.merchant_id inside = %q, want %q", inside.merchantID, merchantB)
	}
}
