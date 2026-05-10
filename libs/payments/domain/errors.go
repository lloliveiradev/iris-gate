package domain

import "errors"

var (
	ErrValidation              = errors.New("validation")
	ErrPaymentIntentNotFound   = errors.New("payment intent not found")
	ErrIdempotencyConflict     = errors.New("idempotency conflict")
	ErrAcquirerRejected        = errors.New("acquirer rejected")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
)
