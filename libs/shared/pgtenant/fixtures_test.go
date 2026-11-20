package pgtenant_test

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Fake merchant ids shared by the unit and integration tests.
const (
	merchantA   = "6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b"
	merchantB   = "0b9e8d7c-6a5b-4c3d-9e2f-1a0b9c8d7e6f"
	nilMerchant = "00000000-0000-0000-0000-000000000000"
)

// wantSetMerchantSQL pins the statement row-level security depends on:
// transaction-local (third argument true) and parameterized.
const wantSetMerchantSQL = "SELECT set_config('app.merchant_id', $1, true)"

// Steps the fakes record, in call order.
const (
	stepBegin    = "begin"
	stepExec     = "exec"
	stepFn       = "fn"
	stepCommit   = "commit"
	stepRollback = "rollback"
)

var (
	errBegin    = errors.New("fake begin failure")
	errSet      = errors.New("fake set_config failure")
	errFn       = errors.New("fake fn failure")
	errCommit   = errors.New("fake commit failure")
	errRollback = errors.New("fake rollback failure")
)

// fakePanic is a distinct type, so a recovered value equal to panicValue can
// only be the one fn raised.
type fakePanic string

const panicValue fakePanic = "fake panic"

// recorder keeps the ordered steps of every call made through one fakeDB.
type recorder struct {
	steps []string
}

func (r *recorder) record(step string) {
	r.steps = append(r.steps, step)
}

// txConfig sets how the fake transactions fail.
type txConfig struct {
	execErr     error
	commitErr   error
	rollbackErr error
	// commitLeavesTxOpen simulates a driver that keeps the transaction open
	// when Commit fails; pgx closes it.
	commitLeavesTxOpen bool
}

// fakeDB hands out one fakeTx per BeginTx call.
type fakeDB struct {
	rec      *recorder
	beginErr error
	tx       txConfig
	txs      []*fakeTx
	begins   int
}

func newFakeDB() *fakeDB {
	return &fakeDB{rec: &recorder{}}
}

func (d *fakeDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	d.begins++
	d.rec.record(stepBegin)
	if d.beginErr != nil {
		return nil, d.beginErr
	}
	tx := &fakeTx{rec: d.rec, cfg: d.tx}
	d.txs = append(d.txs, tx)
	return tx, nil
}

// execCall is one statement run through fakeTx.Exec.
type execCall struct {
	sql  string
	args []any
}

// fakeTx follows the pgx.Tx contract for Exec, Commit and Rollback. The
// embedded nil pgx.Tx makes any other method panic, so the helper cannot
// depend on one unnoticed.
type fakeTx struct {
	pgx.Tx
	rec       *recorder
	cfg       txConfig
	execs     []execCall
	closed    bool
	committed bool
}

func (t *fakeTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.rec.record(stepExec)
	if t.closed {
		return pgconn.CommandTag{}, pgx.ErrTxClosed
	}
	t.execs = append(t.execs, execCall{sql: sql, args: args})
	return pgconn.NewCommandTag("SELECT 1"), t.cfg.execErr
}

func (t *fakeTx) Commit(context.Context) error {
	t.rec.record(stepCommit)
	if t.closed {
		return pgx.ErrTxClosed
	}
	if t.cfg.commitErr != nil {
		t.closed = !t.cfg.commitLeavesTxOpen
		return t.cfg.commitErr
	}
	t.closed, t.committed = true, true
	return nil
}

func (t *fakeTx) Rollback(context.Context) error {
	t.rec.record(stepRollback)
	if t.closed {
		return pgx.ErrTxClosed
	}
	t.closed = true
	return t.cfg.rollbackErr
}
