package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

const TopicPaymentIntentCapturedV1 = "payments.payment_intent.captured.v1"

type CapturePaymentUseCase struct {
	repo  PaymentIntentRepo
	clock Clock
	ids   IDGen
}

func NewCapturePaymentUseCase(repo PaymentIntentRepo, clock Clock, ids IDGen) *CapturePaymentUseCase {
	return &CapturePaymentUseCase{repo: repo, clock: clock, ids: ids}
}

// Execute captura um intent autorizado.
// Idempotente: chamar duas vezes não duplica eventos (segunda chamada
// observa Status=captured e short-circuita).
func (uc *CapturePaymentUseCase) Execute(ctx context.Context, id uuid.UUID) (domain.PaymentIntent, error) {
	pi, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return domain.PaymentIntent{}, err
	}

	if pi.Status == domain.PaymentIntentCaptured {
		return pi, nil
	}

	now := uc.clock.Now()
	if err := pi.MarkCaptured(now); err != nil {
		return domain.PaymentIntent{}, err
	}

	ev := domain.PaymentIntentCapturedV1{
		EventID:         uc.ids.NewUUID(),
		OccurredAt:      now,
		PaymentIntentID: pi.ID,
		MerchantID:      pi.MerchantID,
		AmountCents:     pi.Amount.AmountCents,
		Currency:        pi.Amount.Currency,
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return domain.PaymentIntent{}, fmt.Errorf("marshal event: %w", err)
	}
	msg := OutboxMessage{
		Topic:   TopicPaymentIntentCapturedV1,
		Key:     pi.MerchantID.String(),
		Payload: payload,
		Headers: map[string]string{
			"event-id":     ev.EventID.String(),
			"event-type":   TopicPaymentIntentCapturedV1,
			"published-at": now.Format("2006-01-02T15:04:05Z07:00"),
		},
	}
	if err := uc.repo.UpdateWithOutbox(ctx, pi, msg); err != nil {
		return domain.PaymentIntent{}, err
	}
	return pi, nil
}
