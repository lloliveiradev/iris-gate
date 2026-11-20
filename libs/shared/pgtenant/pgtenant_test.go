package pgtenant_test

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iris-gate/iris-gate/libs/shared/pgtenant"
)

// The production pool must satisfy the port.
var _ pgtenant.TxBeginner = (*pgxpool.Pool)(nil)

// cancelPoint says when a test cancels the context.
type cancelPoint int

const (
	cancelNever cancelPoint = iota
	cancelBeforeBegin
	cancelDuringFn
)

func TestWithMerchant(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		beginErr    error
		tx          txConfig
		cancel      cancelPoint
		fnErr       error
		wantSteps   []string
		wantErrs    []error
		wantNotErrs []error
		wantMessage string
		wantCommit  bool
	}{
		{
			name:       "sets the merchant, runs fn and commits",
			wantSteps:  []string{stepBegin, stepExec, stepFn, stepCommit},
			wantCommit: true,
		},
		{
			name:        "begin failure is wrapped and fn never runs",
			beginErr:    errBegin,
			wantSteps:   []string{stepBegin},
			wantErrs:    []error{errBegin},
			wantMessage: "begin tenant transaction",
		},
		{
			name:        "context done before begin never begins",
			cancel:      cancelBeforeBegin,
			wantErrs:    []error{context.Canceled},
			wantMessage: "begin tenant transaction",
		},
		{
			name:        "set_config failure rolls back and fn never runs",
			tx:          txConfig{execErr: errSet},
			wantSteps:   []string{stepBegin, stepExec, stepRollback},
			wantErrs:    []error{errSet},
			wantMessage: "set tenant",
		},
		{
			name:      "set_config and rollback failures are both returned",
			tx:        txConfig{execErr: errSet, rollbackErr: errRollback},
			wantSteps: []string{stepBegin, stepExec, stepRollback},
			wantErrs:  []error{errSet, errRollback},
		},
		{
			name:      "fn error rolls back",
			fnErr:     errFn,
			wantSteps: []string{stepBegin, stepExec, stepFn, stepRollback},
			wantErrs:  []error{errFn},
		},
		{
			name:        "rollback failure is joined to the fn error",
			fnErr:       errFn,
			tx:          txConfig{rollbackErr: errRollback},
			wantSteps:   []string{stepBegin, stepExec, stepFn, stepRollback},
			wantErrs:    []error{errFn, errRollback},
			wantMessage: "rollback tenant transaction",
		},
		{
			name:        "context done during fn rolls back instead of committing",
			cancel:      cancelDuringFn,
			wantSteps:   []string{stepBegin, stepExec, stepFn, stepRollback},
			wantErrs:    []error{context.Canceled},
			wantMessage: "commit tenant transaction",
		},
		{
			name:        "commit failure is wrapped and the closed tx is not reported",
			tx:          txConfig{commitErr: errCommit},
			wantSteps:   []string{stepBegin, stepExec, stepFn, stepCommit, stepRollback},
			wantErrs:    []error{errCommit},
			wantNotErrs: []error{pgx.ErrTxClosed},
			wantMessage: "commit tenant transaction",
		},
		{
			name:        "commit failure that leaves the tx open is rolled back",
			tx:          txConfig{commitErr: errCommit, commitLeavesTxOpen: true},
			wantSteps:   []string{stepBegin, stepExec, stepFn, stepCommit, stepRollback},
			wantErrs:    []error{errCommit},
			wantMessage: "commit tenant transaction",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := newFakeDB()
			db.beginErr, db.tx = tc.beginErr, tc.tx
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel == cancelBeforeBegin {
				cancel()
			}

			err := pgtenant.WithMerchant(ctx, db, merchantA, func(_ context.Context, tx pgx.Tx) error {
				db.rec.record(stepFn)
				if tx != pgx.Tx(db.txs[0]) {
					t.Error("fn did not get the transaction WithMerchant opened")
				}
				if tc.cancel == cancelDuringFn {
					cancel()
				}
				return tc.fnErr
			})

			if !slices.Equal(db.rec.steps, tc.wantSteps) {
				t.Errorf("steps = %v, want %v", db.rec.steps, tc.wantSteps)
			}
			if tc.wantErrs == nil && err != nil {
				t.Errorf("err = %v, want nil", err)
			}
			for _, want := range tc.wantErrs {
				if !errors.Is(err, want) {
					t.Errorf("err = %v, want it to wrap %v", err, want)
				}
			}
			for _, notWant := range tc.wantNotErrs {
				if errors.Is(err, notWant) {
					t.Errorf("err = %v, must not wrap %v", err, notWant)
				}
			}
			if tc.wantMessage != "" && (err == nil || !strings.Contains(err.Error(), tc.wantMessage)) {
				t.Errorf("err = %v, want message containing %q", err, tc.wantMessage)
			}
			for i, tx := range db.txs {
				if !tx.closed {
					t.Errorf("transaction %d left open", i)
				}
				if tx.committed != tc.wantCommit {
					t.Errorf("transaction %d committed = %v, want %v", i, tx.committed, tc.wantCommit)
				}
			}
		})
	}
}

func TestWithMerchant_ReturnsFnErrorAsIs(t *testing.T) {
	t.Parallel()
	db := newFakeDB()

	err := pgtenant.WithMerchant(t.Context(), db, merchantA, func(context.Context, pgx.Tx) error {
		return errFn
	})

	// Identity, not errors.Is: callers may compare their sentinels with ==.
	if err != errFn {
		t.Fatalf("err = %v, want the fn error itself", err)
	}
}

func TestWithMerchant_SetsMerchantFirstAsBindParameter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		merchantID string
		wantArg    string
	}{
		{name: "canonical id is passed through", merchantID: merchantA, wantArg: merchantA},
		{name: "upper-case id is normalized to lower case", merchantID: strings.ToUpper(merchantA), wantArg: merchantA},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := newFakeDB()

			err := pgtenant.WithMerchant(t.Context(), db, tc.merchantID, func(context.Context, pgx.Tx) error {
				if len(db.txs[0].execs) != 1 {
					t.Errorf("statements before fn = %d, want only set_config", len(db.txs[0].execs))
				}
				return nil
			})
			if err != nil {
				t.Fatalf("WithMerchant: %v", err)
			}

			first := db.txs[0].execs[0]
			if first.sql != wantSetMerchantSQL {
				t.Errorf("first statement = %q, want %q", first.sql, wantSetMerchantSQL)
			}
			if !slices.Equal(first.args, []any{tc.wantArg}) {
				t.Errorf("set_config args = %v, want [%s]", first.args, tc.wantArg)
			}
		})
	}
}

func TestWithMerchant_SequentialCallsUseTheirOwnMerchant(t *testing.T) {
	t.Parallel()
	db := newFakeDB()
	merchants := []string{merchantA, merchantB}
	noop := func(_ context.Context, _ pgx.Tx) error {
		db.rec.record(stepFn)
		return nil
	}

	for _, merchantID := range merchants {
		if err := pgtenant.WithMerchant(t.Context(), db, merchantID, noop); err != nil {
			t.Fatalf("WithMerchant(%s): %v", merchantID, err)
		}
	}

	oneCall := []string{stepBegin, stepExec, stepFn, stepCommit}
	if want := slices.Concat(oneCall, oneCall); !slices.Equal(db.rec.steps, want) {
		t.Errorf("steps = %v, want %v", db.rec.steps, want)
	}
	for i, merchantID := range merchants {
		if got := db.txs[i].execs[0].args; !slices.Equal(got, []any{merchantID}) {
			t.Errorf("call %d set_config args = %v, want [%s]", i, got, merchantID)
		}
	}
}

func TestWithMerchant_RejectsInvalidMerchantIDBeforeBegin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		merchantID string
	}{
		{name: "empty", merchantID: ""},
		{name: "not a uuid", merchantID: "not-a-uuid"},
		{name: "nil uuid", merchantID: nilMerchant},
		{name: "non-hex digit", merchantID: merchantA[:len(merchantA)-1] + "g"},
		{name: "surrounding space", merchantID: " " + merchantA},
		{name: "braced form", merchantID: "{" + merchantA + "}"},
		{name: "urn form", merchantID: "urn:uuid:" + merchantA},
		{name: "unhyphenated form", merchantID: strings.ReplaceAll(merchantA, "-", "")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := newFakeDB()

			err := pgtenant.WithMerchant(t.Context(), db, tc.merchantID, func(context.Context, pgx.Tx) error {
				t.Error("fn ran for an invalid merchant id")
				return nil
			})

			if !errors.Is(err, pgtenant.ErrInvalidMerchantID) {
				t.Errorf("err = %v, want ErrInvalidMerchantID", err)
			}
			if db.begins != 0 {
				t.Errorf("BeginTx calls = %d, want 0", db.begins)
			}
		})
	}
}

func TestWithMerchant_PanicRollsBackAndRepanics(t *testing.T) {
	t.Parallel()
	db := newFakeDB()
	defer func() {
		if got := recover(); got != panicValue {
			t.Fatalf("recovered %v, want the original panic value", got)
		}
		want := []string{stepBegin, stepExec, stepFn, stepRollback}
		if !slices.Equal(db.rec.steps, want) {
			t.Errorf("steps = %v, want %v", db.rec.steps, want)
		}
		if tx := db.txs[0]; !tx.closed || tx.committed {
			t.Errorf("transaction closed = %v, committed = %v; want rolled back", tx.closed, tx.committed)
		}
	}()

	err := pgtenant.WithMerchant(t.Context(), db, merchantA, func(context.Context, pgx.Tx) error {
		db.rec.record(stepFn)
		panic(panicValue)
	})
	t.Fatalf("WithMerchant returned %v instead of re-panicking", err)
}

// A test helper calling t.Fatal inside fn ends the goroutine with
// runtime.Goexit; the transaction must still end so the pool connection is
// released.
func TestWithMerchant_GoexitRollsBack(t *testing.T) {
	t.Parallel()
	db := newFakeDB()
	done := make(chan struct{})

	go func() {
		defer close(done)
		_ = pgtenant.WithMerchant(t.Context(), db, merchantA, func(context.Context, pgx.Tx) error {
			runtime.Goexit()
			return nil
		})
	}()
	<-done

	if tx := db.txs[0]; !tx.closed || tx.committed {
		t.Errorf("transaction closed = %v, committed = %v; want rolled back", tx.closed, tx.committed)
	}
}

// fn gets a pgx.Tx and WithMerchant takes a TxBeginner; keeping pgx.Tx out
// of TxBeginner is what makes nesting unrepresentable.
func TestTxBeginner_IsNotSatisfiedByTx(t *testing.T) {
	t.Parallel()
	if reflect.TypeFor[pgx.Tx]().Implements(reflect.TypeFor[pgtenant.TxBeginner]()) {
		t.Fatal("pgx.Tx satisfies TxBeginner: fn could nest WithMerchant on its tx")
	}
}
