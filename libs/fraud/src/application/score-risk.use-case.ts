import { Decision } from '../domain/decision';
import { DuplicateAssessmentError, IncompleteAssessmentError } from '../domain/errors';
import { FraudReason } from '../domain/reason';
import { RiskAssessment } from '../domain/risk-assessment';
import { applyTestScenario, RiskVerdict, TestScenario } from '../domain/test-scenario';
import { DecisionThresholds } from '../domain/thresholds';
import { evaluateRules } from './evaluate-rules';
import { createFeatureSnapshot, FeatureSnapshot, FeatureSnapshotReader } from './feature-snapshot';
import { Clock, IDGen, OutboxEventInput } from './ports';
import { RegisteredRule } from './rule-registry';
import { AttemptedOutcome, ScoredAssessmentStore } from './score-risk.ports';

/** Topic of the event written to the outbox for every new assessment. */
export const ASSESSMENT_COMPLETED_TOPIC = 'fraud.assessment.completed.v1';

/** A scoring request; mirrors the `fraud-internal` `RiskAssessmentRequest`. */
export interface ScoreRiskInput {
  paymentIntentId: string;
  merchantId: string;
  /** Integer minor units; any safe integer above zero, including above 2^31. */
  amountCents: number;
  currency: string;
  cardFingerprint: string;
  cardBrand: string;
  customerRef?: string;
  /** Honored only when the use case runs with test hooks enabled. */
  testScenario?: TestScenario;
}

export interface ScoreRiskOutput {
  assessmentId: string;
  decision: Decision;
  score: number;
  reasons: FraudReason[];
  /** True when a stored assessment answered the request and nothing was written. */
  deduplicated: boolean;
}

/**
 * Scores a payment intent synchronously, once per `payment_intent_id`.
 *
 * - A repeat returns the stored assessment without rescoring or writing anything.
 * - A new request loads one feature snapshot, runs the rules and stores the
 *   assessment, the snapshot, the `attempted` outcome and the outbox event together.
 * - Losing the insert race to a concurrent request (`DuplicateAssessmentError`)
 *   returns the winner's stored assessment.
 * - Fails closed: any error loading features, scoring or saving propagates; there is
 *   no fallback decision.
 *
 * `testHooksEnabled` defaults to `false`; while it is off, `testScenario` is ignored.
 */
export class ScoreRiskUseCase {
  constructor(
    private readonly store: ScoredAssessmentStore,
    private readonly featureReader: FeatureSnapshotReader,
    private readonly rules: readonly RegisteredRule[],
    private readonly thresholds: DecisionThresholds,
    private readonly clock: Clock,
    private readonly idGen: IDGen,
    private readonly testHooksEnabled = false,
  ) {}

  async execute(input: ScoreRiskInput): Promise<ScoreRiskOutput> {
    const requestedAt = this.clock.now();
    // Created first so malformed input is rejected before any I/O.
    const assessment = RiskAssessment.create({
      id: this.idGen.newId(),
      paymentIntentId: input.paymentIntentId,
      merchantId: input.merchantId,
      accountRef: input.cardFingerprint,
      amountCents: input.amountCents,
      currency: input.currency,
      requestedAt,
    });

    const stored = await this.store.findByPaymentIntentId(input.paymentIntentId);
    if (stored) return storedOutput(stored);

    // Validated and frozen here, so the snapshot stored is exactly what the rules saw.
    const features = createFeatureSnapshot(
      await this.featureReader.load({
        cardFingerprint: input.cardFingerprint,
        customerRef: input.customerRef,
      }),
    );
    const verdict = this.decide(input, features, requestedAt);
    const completedAt = this.clock.now();
    assessment.complete(verdict.decision, verdict.score, [...verdict.reasons], completedAt);

    try {
      await this.store.saveScored({
        assessment,
        features,
        attempt: attemptedOutcome(input, requestedAt),
        outboxEvent: this.completedEvent(assessment, completedAt),
      });
    } catch (error) {
      if (!(error instanceof DuplicateAssessmentError)) throw error;
      return this.storedAfterLostRace(input.paymentIntentId, error);
    }

    return {
      assessmentId: assessment.id,
      decision: verdict.decision,
      score: verdict.score,
      reasons: [...verdict.reasons],
      deduplicated: false,
    };
  }

  private decide(input: ScoreRiskInput, features: FeatureSnapshot, now: Date): RiskVerdict {
    const ruleVerdict = evaluateRules(
      this.rules,
      { features, amountCents: input.amountCents, now },
      this.thresholds,
    );
    if (!this.testHooksEnabled || input.testScenario === undefined) return ruleVerdict;
    return applyTestScenario(ruleVerdict, input.testScenario, this.thresholds);
  }

  private async storedAfterLostRace(
    paymentIntentId: string,
    duplicate: DuplicateAssessmentError,
  ): Promise<ScoreRiskOutput> {
    const winner = await this.store.findByPaymentIntentId(paymentIntentId);
    if (!winner) throw duplicate;
    return storedOutput(winner);
  }

  private completedEvent(assessment: RiskAssessment, completedAt: Date): OutboxEventInput {
    const eventId = this.idGen.newId();
    return {
      eventId,
      eventType: ASSESSMENT_COMPLETED_TOPIC,
      topic: ASSESSMENT_COMPLETED_TOPIC,
      occurredAt: completedAt,
      payload: {
        event_id: eventId,
        occurred_at: completedAt.toISOString(),
        assessment_id: assessment.id,
        payment_intent_id: assessment.paymentIntentId,
        merchant_id: assessment.merchantId,
        decision: assessment.decision,
        score: assessment.score,
        reasons: assessment.reasons.map((reason) => ({
          code: reason.code,
          weight: reason.weight,
          score: reason.score,
          description: reason.description,
        })),
      },
    };
  }
}

function attemptedOutcome(input: ScoreRiskInput, occurredAt: Date): AttemptedOutcome {
  return {
    paymentIntentId: input.paymentIntentId,
    merchantId: input.merchantId,
    cardFingerprint: input.cardFingerprint,
    cardBrand: input.cardBrand,
    customerRef: input.customerRef ?? null,
    amountCents: input.amountCents,
    currency: input.currency,
    occurredAt,
  };
}

/** Answers from a stored assessment; one without a decision fails instead of approving. */
function storedOutput(assessment: RiskAssessment): ScoreRiskOutput {
  if (assessment.decision === null) {
    throw new IncompleteAssessmentError(
      `assessment ${assessment.id} for payment intent ${assessment.paymentIntentId} has no decision (${assessment.status})`,
    );
  }
  return {
    assessmentId: assessment.id,
    decision: assessment.decision,
    score: assessment.score,
    reasons: [...assessment.reasons],
    deduplicated: true,
  };
}
