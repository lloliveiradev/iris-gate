package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

const TopicPaymentIntentAuthorizedV1 = "payments.payment_intent.authorized.v1"

type AuthorizePaymentUseCase struct {
	repo     PaymentIntentRepo
	acquirer Acquirer
	clock    Clock
	ids      IDGen
}

func NewAuthorizePaymentUseCase(repo PaymentIntentRepo, acquirer Acquirer, clock Clock, ids IDGen) *AuthorizePaymentUseCase {
	return &AuthorizePaymentUseCase{repo: repo, acquirer: acquirer, clock: clock, ids: ids}
}

// Execute authorizes an existing intent:
//
//  1. Loads it from the repo, scoped to the merchant.
//  2. Checks the status (pending; idempotent when already authorized).
//  3. Chama Acquirer.Authorize.
//  4. Applies the status transition and writes the event to the outbox (same tx).
//
// Errors:
//   - ErrPaymentIntentNotFound — id does not exist or belongs to another merchant
//   - ErrInvalidStatusTransition — intent already captured or canceled
//   - ErrAcquirerRejected — the acquirer declined
func (uc *AuthorizePaymentUseCase) Execute(ctx context.Context, merchantID, id uuid.UUID) (domain.PaymentIntent, error) {
	pi, err := uc.repo.GetByID(ctx, merchantID, id)
	if err != nil {
		return domain.PaymentIntent{}, err
	}

	// Idempotent: when already authorized, return without calling the acquirer again.
	if pi.Status == domain.PaymentIntentAuthorized {
		return pi, nil
	}

	out, err := uc.acquirer.Authorize(ctx, AuthorizeInput{
		IntentID:    pi.ID,
		AmountCents: pi.Amount.AmountCents,
		Currency:    pi.Amount.Currency,
		CardToken:   pi.CardToken,
	})
	if err != nil {
		return domain.PaymentIntent{}, fmt.Errorf("acquirer authorize: %w", err)
	}

	if !out.Approved {
		if err := pi.MarkFailed(); err != nil {
			return domain.PaymentIntent{}, err
		}
		// The decline is not persisted yet: the failed event and its outbox row
		// belong to a later task (D3).
		return pi, fmt.Errorf("%w: %s", domain.ErrAcquirerRejected, out.Reason)
	}

	now := uc.clock.Now()
	if err := pi.MarkAuthorized(now); err != nil {
		return domain.PaymentIntent{}, err
	}

	ev := domain.PaymentIntentAuthorizedV1{
		EventID:         uc.ids.NewUUID(),
		OccurredAt:      now,
		PaymentIntentID: pi.ID,
		MerchantID:      pi.MerchantID,
		AcquirerRef:     out.AcquirerRef,
		AmountCents:     pi.Amount.AmountCents,
		Currency:        pi.Amount.Currency,
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return domain.PaymentIntent{}, fmt.Errorf("marshal event: %w", err)
	}
	msg := OutboxMessage{
		Topic:   TopicPaymentIntentAuthorizedV1,
		Key:     pi.MerchantID.String(),
		Payload: payload,
		Headers: map[string]string{
			"event-id":     ev.EventID.String(),
			"event-type":   TopicPaymentIntentAuthorizedV1,
			"published-at": now.Format("2006-01-02T15:04:05Z07:00"),
		},
	}
	if err := uc.repo.UpdateWithOutbox(ctx, pi, msg); err != nil {
		return domain.PaymentIntent{}, err
	}
	return pi, nil
}
