// Package acquirer holds the acquirer adapters. Fake approves by default and
// serves dev and tests; HTTPAcquirer talks to a real processor once sandbox
// credentials exist.
package acquirer

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/application"
)

// ErrIdempotencyKeyRequired is returned when AuthorizeInput.IdempotencyKey is empty.
var ErrIdempotencyKeyRequired = errors.New("acquirer: idempotency key required")

// Fake reasons, reported in AuthorizeOutput.Reason.
const (
	fakeApprovedReason = "fake-acquirer-approved"
	fakeDeclinedReason = "fake-acquirer-declined"
	fakeRefPrefix      = "fake_"
)

// Fake is an in-memory acquirer. It approves every card except the declined
// tokens given to NewFake, and honors the port's idempotency contract: the
// first call for a key decides, and every later call with that key returns the
// same result (same AcquirerRef) without a new authorization. Results are kept
// for the life of the process, which suits dev and tests only.
type Fake struct {
	declined map[string]struct{}

	mu      sync.Mutex
	results map[string]application.AuthorizeOutput
}

// NewFake returns a Fake that declines the given card tokens and approves any
// other. With no argument it approves everything.
func NewFake(declinedCardTokens ...string) *Fake {
	declined := make(map[string]struct{}, len(declinedCardTokens))
	for _, token := range declinedCardTokens {
		declined[token] = struct{}{}
	}
	return &Fake{declined: declined, results: map[string]application.AuthorizeOutput{}}
}

// Authorize decides on the first call for in.IdempotencyKey and replays that
// decision afterwards.
func (f *Fake) Authorize(_ context.Context, in application.AuthorizeInput) (application.AuthorizeOutput, error) {
	if in.IdempotencyKey == "" {
		return application.AuthorizeOutput{}, ErrIdempotencyKeyRequired
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if out, ok := f.results[in.IdempotencyKey]; ok {
		return out, nil
	}
	out := application.AuthorizeOutput{AcquirerRef: fakeRefPrefix + uuid.NewString(), Approved: true, Reason: fakeApprovedReason}
	if _, ok := f.declined[in.CardToken]; ok {
		out.Approved, out.Reason = false, fakeDeclinedReason
	}
	f.results[in.IdempotencyKey] = out
	return out, nil
}

// Authorizations counts distinct authorizations, that is first calls per
// idempotency key; replays are not counted.
func (f *Fake) Authorizations() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.results)
}
