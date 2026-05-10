package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

const TopicPaymentIntentCreatedV1 = "payments.payment_intent.created.v1"

type CreatePaymentIntentInput struct {
	MerchantID  uuid.UUID
	Amount      domain.Money
	CardToken   string
	Description string
}

type CreatePaymentIntentUseCase struct {
	repo  PaymentIntentRepo
	clock Clock
	ids   IDGen
}

func NewCreatePaymentIntentUseCase(repo PaymentIntentRepo, clock Clock, ids IDGen) *CreatePaymentIntentUseCase {
	return &CreatePaymentIntentUseCase{repo: repo, clock: clock, ids: ids}
}

// Execute cria um PaymentIntent novo + enfileira o evento na outbox **na
// mesma transação** (responsabilidade do repo).
//
// Idempotência é tratada pelo middleware HTTP — não é responsabilidade deste
// use case. Se o middleware chamar Execute, é porque a chave é nova.
func (uc *CreatePaymentIntentUseCase) Execute(ctx context.Context, in CreatePaymentIntentInput) (domain.PaymentIntent, error) {
	pi, err := domain.NewPaymentIntent(uc.ids.NewUUID(), in.MerchantID, in.Amount, in.CardToken, in.Description, uc.clock.Now())
	if err != nil {
		return domain.PaymentIntent{}, err
	}

	ev := domain.PaymentIntentCreatedV1{
		EventID:         uc.ids.NewUUID(),
		OccurredAt:      uc.clock.Now(),
		PaymentIntentID: pi.ID,
		MerchantID:      pi.MerchantID,
		AmountCents:     pi.Amount.AmountCents,
		Currency:        pi.Amount.Currency,
		Status:          string(pi.Status),
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return domain.PaymentIntent{}, fmt.Errorf("marshal event: %w", err)
	}

	msg := OutboxMessage{
		Topic:   TopicPaymentIntentCreatedV1,
		Key:     pi.MerchantID.String(),
		Payload: payload,
		Headers: map[string]string{
			"event-id":      ev.EventID.String(),
			"event-type":    TopicPaymentIntentCreatedV1,
			"published-at":  ev.OccurredAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}

	if err := uc.repo.CreateWithOutbox(ctx, pi, msg); err != nil {
		// Bubble up tipo do erro pra mapeamento HTTP cuidar do status code.
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			return domain.PaymentIntent{}, err
		}
		return domain.PaymentIntent{}, fmt.Errorf("persist intent: %w", err)
	}
	return pi, nil
}
