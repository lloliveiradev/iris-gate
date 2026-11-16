package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

// Topics written to the outbox by the authorize use case.
const (
	TopicPaymentIntentAuthorizedV1 = "payments.payment_intent.authorized.v1"
	TopicPaymentIntentFailedV1     = "payments.payment_intent.failed.v1"
)

// AuthorizePaymentUseCase asks the acquirer to authorize a pending intent and
// persists the decision.
type AuthorizePaymentUseCase struct {
	repo     PaymentIntentRepo
	acquirer Acquirer
	clock    Clock
	ids      IDGen
}

// NewAuthorizePaymentUseCase wires the use case with its ports.
func NewAuthorizePaymentUseCase(repo PaymentIntentRepo, acquirer Acquirer, clock Clock, ids IDGen) *AuthorizePaymentUseCase {
	return &AuthorizePaymentUseCase{repo: repo, acquirer: acquirer, clock: clock, ids: ids}
}

// Execute authorizes an existing intent:
//
//  1. Loads it from the repo, scoped to the merchant.
//  2. Checks the status: pending goes on; already authorized returns as is
//     (idempotent); any other status fails before the acquirer is called.
//  3. Calls Acquirer.Authorize with the intent id as the idempotency key, so
//     concurrent or retried calls for one intent are one acquirer operation.
//  4. Persists the decision (authorized, or failed with a failure code) and
//     its event in one transaction, conditional on the loaded version.
//
// Errors:
//   - ErrPaymentIntentNotFound: the id does not exist or belongs to another merchant
//   - ErrInvalidStatusTransition: the intent is captured, failed or canceled
//   - ErrConcurrentUpdate: another request changed the intent first; nothing was written
//   - ErrAcquirerRejected: the acquirer declined; the returned intent is the
//     persisted failed intent
func (uc *AuthorizePaymentUseCase) Execute(ctx context.Context, merchantID, id uuid.UUID) (domain.PaymentIntent, error) {
	pi, err := uc.repo.GetByID(ctx, merchantID, id)
	if err != nil {
		return domain.PaymentIntent{}, err
	}

	needed, err := pi.AuthorizationNeeded()
	if err != nil {
		return domain.PaymentIntent{}, err
	}
	if !needed {
		return pi, nil
	}

	out, err := uc.acquirer.Authorize(ctx, AuthorizeInput{
		IntentID:       pi.ID,
		AmountCents:    pi.Amount.AmountCents,
		Currency:       pi.Amount.Currency,
		CardToken:      pi.CardToken,
		IdempotencyKey: pi.ID.String(),
	})
	if err != nil {
		return domain.PaymentIntent{}, fmt.Errorf("acquirer authorize: %w", err)
	}

	if !out.Approved {
		return uc.persistDecline(ctx, pi, out.Reason)
	}
	return uc.persistApproval(ctx, pi, out.AcquirerRef)
}

func (uc *AuthorizePaymentUseCase) persistApproval(ctx context.Context, pi domain.PaymentIntent, acquirerRef string) (domain.PaymentIntent, error) {
	now := uc.clock.Now()
	if err := pi.MarkAuthorized(now); err != nil {
		return domain.PaymentIntent{}, err
	}
	ev := domain.PaymentIntentAuthorizedV1{
		EventID:         uc.ids.NewUUID(),
		OccurredAt:      now,
		PaymentIntentID: pi.ID,
		MerchantID:      pi.MerchantID,
		AcquirerRef:     acquirerRef,
		AmountCents:     pi.Amount.AmountCents,
		Currency:        pi.Amount.Currency,
	}
	msg, err := newOutboxMessage(TopicPaymentIntentAuthorizedV1, pi.MerchantID, ev.EventID, now, ev)
	if err != nil {
		return domain.PaymentIntent{}, err
	}
	if err := uc.repo.UpdateWithOutbox(ctx, pi, msg); err != nil {
		return domain.PaymentIntent{}, fmt.Errorf("persist authorization: %w", err)
	}
	return pi, nil
}

// persistDecline saves the intent as failed with its failed event, then still
// reports the decline as ErrAcquirerRejected.
func (uc *AuthorizePaymentUseCase) persistDecline(ctx context.Context, pi domain.PaymentIntent, reason string) (domain.PaymentIntent, error) {
	rejected := fmt.Errorf("%w: %s", domain.ErrAcquirerRejected, reason)
	now := uc.clock.Now()
	ev, err := pi.MarkFailed(domain.FailureCardDeclined, uc.ids.NewUUID(), now)
	if err != nil {
		return domain.PaymentIntent{}, err
	}
	if ev == nil {
		// Already failed: nothing new to persist.
		return pi, rejected
	}
	msg, err := newOutboxMessage(TopicPaymentIntentFailedV1, pi.MerchantID, ev.EventID, now, ev)
	if err != nil {
		return domain.PaymentIntent{}, err
	}
	if err := uc.repo.UpdateWithOutbox(ctx, pi, msg); err != nil {
		return domain.PaymentIntent{}, fmt.Errorf("persist decline: %w", err)
	}
	return pi, rejected
}

// newOutboxMessage serializes an event keyed by merchant, with the common
// event headers of the AsyncAPI contract.
func newOutboxMessage(topic string, merchantID, eventID uuid.UUID, at time.Time, event any) (OutboxMessage, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return OutboxMessage{}, fmt.Errorf("marshal event: %w", err)
	}
	return OutboxMessage{
		Topic:   topic,
		Key:     merchantID.String(),
		Payload: payload,
		Headers: map[string]string{
			"event-id":     eventID.String(),
			"event-type":   topic,
			"published-at": at.Format(time.RFC3339),
		},
	}, nil
}
