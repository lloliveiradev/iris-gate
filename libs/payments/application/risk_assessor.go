package application

import (
	"context"

	"github.com/google/uuid"

	"github.com/iris-gate/iris-gate/libs/payments/domain"
)

// RiskDecision is the anti-fraud verdict for a payment intent.
type RiskDecision string

// Risk decisions returned by the scoring service.
const (
	RiskDecisionApprove RiskDecision = "APPROVE"
	RiskDecisionReview  RiskDecision = "REVIEW"
	RiskDecisionReject  RiskDecision = "REJECT"
)

// RiskReasonUnavailable is the reason recorded when the scoring service could
// not produce a trustworthy answer (timeout, network error, non-2xx status or
// invalid body).
const RiskReasonUnavailable = "RISK_UNAVAILABLE"

// RiskAssessmentRequest carries what the scoring service needs to assess one
// payment intent. It holds an opaque card fingerprint, never a PAN.
type RiskAssessmentRequest struct {
	PaymentIntentID uuid.UUID
	MerchantID      uuid.UUID
	Amount          domain.Money
	CardFingerprint string
	CardBrand       string
	// CustomerRef is optional; empty means absent.
	CustomerRef string
	// TestScenario is optional; empty means absent. The scoring service only
	// honors it when its test hooks are enabled.
	TestScenario string
}

// RiskSnapshot is the risk outcome recorded for a payment intent.
type RiskSnapshot struct {
	// AssessmentID is uuid.Nil when the scoring service was not reached.
	AssessmentID uuid.UUID
	Decision     RiskDecision
	// Score is in 0..100.
	Score float64
	// Reasons holds the reason codes, for example "VELOCITY".
	Reasons []string
}

// RiskUnavailableSnapshot is the fail-closed outcome: REVIEW with reason
// RISK_UNAVAILABLE. Uncertainty never becomes an APPROVE.
func RiskUnavailableSnapshot() RiskSnapshot {
	return RiskSnapshot{
		Decision: RiskDecisionReview,
		Reasons:  []string{RiskReasonUnavailable},
	}
}

// RiskAssessor scores a payment intent before it reaches the acquirer.
//
// Assess never returns an error: every failure to obtain a valid assessment
// yields RiskUnavailableSnapshot, so a caller cannot fall back to APPROVE by
// mistake.
type RiskAssessor interface {
	Assess(ctx context.Context, req RiskAssessmentRequest) RiskSnapshot
}
