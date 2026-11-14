import { Decision } from '../domain/decision';
import {
  AssessmentConflictError,
  DuplicateAssessmentError,
  IncompleteAssessmentError,
} from '../domain/errors';
import { FraudReason } from '../domain/reason';
import { RiskAssessment } from '../domain/risk-assessment';
import {
  applyTestScenario,
  assertKnownTestScenario,
  RiskVerdict,
  TestScenario,
} from '../domain/test-scenario';
import { DecisionThresholds } from '../domain/thresholds';
import { evaluateRules } from './evaluate-rules';
import { createFeatureSnapshot, FeatureSnapshot, FeatureSnapshotReader } from './feature-snapshot';
import { Clock, IDGen, OutboxEventInput } from './ports';
import { assertBlockedCardRejects, RegisteredRule } from './rule-registry';
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
  /**
   * Milliseconds from `requestedAt` to `completedAt` of the assessment. A repeat
   * reports the latency of the original scoring, not of the lookup.
   */
  latencyMs: number;
}

/**
 * Scores a payment intent synchronously, once per `payment_intent_id`.
 *
 * - A repeat returns the stored assessment without rescoring or writing anything. A
 *   repeat whose `merchant_id`, `amount_cents` or `currency` differs from the stored
 *   assessment throws `AssessmentConflictError`.
 * - A new request loads one feature snapshot, runs the rules and stores the
 *   assessment, the snapshot, the `attempted` outcome and the outbox event together.
 * - Losing the insert race to a concurrent request (`DuplicateAssessmentError`)
 *   returns the winner's stored assessment, with the same conflict check.
 * - Fails closed: any error loading features, scoring or saving propagates; there is
 *   no fallback decision.
 *
 * `testHooksEnabled` defaults to `false`; while it is off, `testScenario` is ignored.
 * The constructor throws `InvalidRuleConfigError` when the rules cannot turn a blocked
 * card into `REJECT` under the given thresholds.
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
  ) {
    assertBlockedCardRejects(rules, thresholds);
  }

  async execute(input: ScoreRiskInput): Promise<ScoreRiskOutput> {
    const requestedAt = this.clock.now();
    // Created first, and the scenario checked, so bad input is rejected before any I/O.
    const assessment = RiskAssessment.create({
      id: this.idGen.newId(),
      paymentIntentId: input.paymentIntentId,
      merchantId: input.merchantId,
      accountRef: input.cardFingerprint,
      amountCents: input.amountCents,
      currency: input.currency,
      requestedAt,
    });
    const testScenario = this.activeTestScenario(input);

    const stored = await this.store.findByPaymentIntentId(input.paymentIntentId);
    if (stored) return storedOutput(stored, input);

    // Validated and frozen here, so the snapshot stored is exactly what the rules saw.
    const features = createFeatureSnapshot(
      await this.featureReader.load({
        cardFingerprint: input.cardFingerprint,
        customerRef: input.customerRef,
      }),
    );
    const verdict = this.decide(input.amountCents, features, requestedAt, testScenario);
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
      return this.storedAfterLostRace(input, error);
    }

    return {
      assessmentId: assessment.id,
      decision: verdict.decision,
      score: verdict.score,
      reasons: [...verdict.reasons],
      deduplicated: false,
      latencyMs: latencyMs(requestedAt, completedAt),
    };
  }

  /** The scenario to apply, validated; `undefined` when hooks are off or none was sent. */
  private activeTestScenario(input: ScoreRiskInput): TestScenario | undefined {
    if (!this.testHooksEnabled || input.testScenario === undefined) return undefined;
    assertKnownTestScenario(input.testScenario);
    return input.testScenario;
  }

  private decide(
    amountCents: number,
    features: FeatureSnapshot,
    now: Date,
    testScenario: TestScenario | undefined,
  ): RiskVerdict {
    const ruleVerdict = evaluateRules(this.rules, { features, amountCents, now }, this.thresholds);
    if (testScenario === undefined) return ruleVerdict;
    return applyTestScenario(ruleVerdict, testScenario, this.thresholds);
  }

  private async storedAfterLostRace(
    input: ScoreRiskInput,
    duplicate: DuplicateAssessmentError,
  ): Promise<ScoreRiskOutput> {
    const winner = await this.store.findByPaymentIntentId(input.paymentIntentId);
    if (!winner) throw duplicate;
    return storedOutput(winner, input);
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

/**
 * Answers a repeat from the stored assessment. A request that does not match it
 * throws `AssessmentConflictError`; a stored assessment without a decision fails
 * instead of approving.
 */
function storedOutput(assessment: RiskAssessment, input: ScoreRiskInput): ScoreRiskOutput {
  const conflicts = conflictingFields(assessment, input);
  if (conflicts.length > 0) {
    throw new AssessmentConflictError(
      `payment intent ${assessment.paymentIntentId} was assessed with a different ${conflicts.join(', ')}`,
    );
  }
  if (assessment.decision === null || assessment.completedAt === null) {
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
    latencyMs: latencyMs(assessment.requestedAt, assessment.completedAt),
  };
}

/** Wire names of the request fields that differ from the stored assessment. */
function conflictingFields(stored: RiskAssessment, input: ScoreRiskInput): string[] {
  const fields: string[] = [];
  if (stored.merchantId !== input.merchantId) fields.push('merchant_id');
  if (stored.amountCents !== input.amountCents) fields.push('amount_cents');
  if (stored.currency !== input.currency) fields.push('currency');
  return fields;
}

function latencyMs(requestedAt: Date, completedAt: Date): number {
  return completedAt.getTime() - requestedAt.getTime();
}
