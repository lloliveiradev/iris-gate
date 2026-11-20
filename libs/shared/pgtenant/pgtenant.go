// Package pgtenant runs Postgres work inside a transaction scoped to one
// merchant. Row-level security policies read the merchant from the
// transaction-local setting app.merchant_id, so every merchant-scoped
// statement must run through WithMerchant.
package pgtenant

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// setMerchantSQL scopes the transaction to one merchant. The third argument
// (is_local = true) makes the setting end with the transaction, so a pooled
// connection never carries it to its next user.
const setMerchantSQL = "SELECT set_config('app.merchant_id', $1, true)"

// canonicalUUIDLength is the length of the hyphenated 8-4-4-4-12 UUID form,
// the only form WithMerchant accepts.
const canonicalUUIDLength = 36

// ErrInvalidMerchantID is returned when the merchant id is empty, is not a
// UUID in the hyphenated 8-4-4-4-12 form, or is the nil UUID. WithMerchant
// returns it before opening a transaction.
var ErrInvalidMerchantID = errors.New("pgtenant: invalid merchant id")

// TxBeginner opens a transaction. *pgxpool.Pool satisfies it.
//
// It asks for BeginTx rather than Begin on purpose: pgx.Tx has Begin (a
// savepoint) but not BeginTx, so the tx handed to fn cannot be passed back
// into WithMerchant.
type TxBeginner interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// WithMerchant opens a transaction on db, sets app.merchant_id to merchantID
// as its first statement (transaction-local), runs fn and commits when fn
// returns nil. It rolls back when anything fails, and on a panic it rolls
// back and re-panics with the same value.
//
// merchantID must be a UUID in the hyphenated form; upper-case hex digits are
// accepted and normalized to the lower-case canonical form before they reach
// the database. Anything else, including the nil UUID, returns
// ErrInvalidMerchantID without opening a transaction.
//
// Inside fn use only the tx you are given: statements run on the pool or on
// another connection are outside the merchant scope. Nesting is not
// supported: do not call WithMerchant from fn, and never pass tx.Conn() as
// db, because a second BEGIN on the same connection would switch the merchant
// and commit the outer transaction early.
//
// fn's error is returned as is, so callers can compare their own sentinel
// errors; a rollback failure is joined to it, never put in its place. A
// context that is done before the commit makes WithMerchant roll back and
// return an error wrapping ctx.Err().
func WithMerchant(
	ctx context.Context,
	db TxBeginner,
	merchantID string,
	fn func(ctx context.Context, tx pgx.Tx) error,
) (err error) {
	tenantID, err := normalizeMerchantID(merchantID)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("begin tenant transaction: %w", err)
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tenant transaction: %w", err)
	}

	committed := false
	// The deferred rollback also covers a panic and runtime.Goexit, so the
	// transaction always ends and the pool gets its connection back.
	defer func() {
		if committed {
			return
		}
		recovered := recover()
		// Rollback uses ctx: when ctx is done, pgx closes the connection and
		// the server aborts the transaction, so nothing hangs or leaks.
		rollbackErr := tx.Rollback(ctx)
		if recovered != nil {
			// A panicking call cannot return the rollback error; the panic is
			// the failure the caller has to see.
			panic(recovered)
		}
		// ErrTxClosed means the driver already ended the transaction, as pgx
		// does when Commit fails.
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback tenant transaction: %w", rollbackErr))
		}
	}()

	if _, err := tx.Exec(ctx, setMerchantSQL, tenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("commit tenant transaction: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant transaction: %w", err)
	}
	committed = true
	return nil
}

// normalizeMerchantID validates merchantID and returns its lower-case
// canonical form.
func normalizeMerchantID(merchantID string) (string, error) {
	// uuid.Parse also accepts the urn:uuid:, braced and unhyphenated forms;
	// the length check keeps only the hyphenated one.
	if len(merchantID) != canonicalUUIDLength {
		return "", ErrInvalidMerchantID
	}
	id, err := uuid.Parse(merchantID)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidMerchantID, err)
	}
	if id == uuid.Nil {
		return "", ErrInvalidMerchantID
	}
	return id.String(), nil
}
