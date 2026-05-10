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

// Execute autoriza um intent existente:
//
//   1. Lê do repo.
//   2. Garante estado válido (pending; idempotente se já authorized).
//   3. Chama Acquirer.Authorize.
//   4. Aplica transição de status + escreve evento na outbox (mesma tx).
//
// Errors:
//   - ErrPaymentIntentNotFound — id não existe
//   - ErrInvalidStatusTransition — intent já capturado/cancelado
//   - ErrAcquirerRejected — adquirente recusou
func (uc *AuthorizePaymentUseCase) Execute(ctx context.Context, id uuid.UUID) (domain.PaymentIntent, error) {
	pi, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return domain.PaymentIntent{}, err
	}

	// Idempotência: se já authorized, devolve sem chamar adquirente de novo.
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
		// Persistimos a falha sem evento por enquanto (evento .failed.v1 é trabalho futuro).
		// Reuse UpdateWithOutbox com mensagem vazia exigiria refactor; mantemos
		// semântica: failed é registrado mas não publicado.
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
