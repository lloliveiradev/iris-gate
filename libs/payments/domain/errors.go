package domain

import "errors"

// Domain errors of the payments context. Infra maps them to HTTP problems.
var (
	ErrValidation              = errors.New("validation")
	ErrPaymentIntentNotFound   = errors.New("payment intent not found")
	ErrIdempotencyConflict     = errors.New("idempotency conflict")
	ErrAcquirerRejected        = errors.New("acquirer rejected")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	// ErrConcurrentUpdate means the intent changed after it was loaded: another
	// request won the optimistic-lock race and nothing was written.
	ErrConcurrentUpdate = errors.New("concurrent update")
)
