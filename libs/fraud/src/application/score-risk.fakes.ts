import { Decision } from '../domain/decision';
import { DuplicateAssessmentError } from '../domain/errors';
import { FraudReason } from '../domain/reason';
import { RiskAssessment, RiskAssessmentSnapshot } from '../domain/risk-assessment';
import { TestScenario } from '../domain/test-scenario';
import { FeatureSnapshot, FeatureSnapshotReader } from './feature-snapshot';
import {
  FIXTURE_CARD_FINGERPRINT,
  FIXTURE_CUSTOMER_REF,
  FIXTURE_NOW,
  NORMAL_AMOUNT_CENTS,
} from './fixtures';
import { ScoredAssessment, ScoredAssessmentStore } from './score-risk.ports';
import { ScoreRiskInput } from './score-risk.use-case';

/** Shared fake values for the scoring tests. Not exported from the package entry point. */
export const FAKE_PAYMENT_INTENT_ID = '00000000-0000-4000-8000-0000000000a1';
export const FAKE_OTHER_PAYMENT_INTENT_ID = '00000000-0000-4000-8000-0000000000a2';
export const FAKE_MERCHANT_ID = '00000000-0000-4000-8000-0000000000b1';
export const FAKE_STORED_ASSESSMENT_ID = '00000000-0000-4000-8000-0000000000c1';
export const FAKE_CARD_FINGERPRINT = FIXTURE_CARD_FINGERPRINT;
export const FAKE_CUSTOMER_REF = FIXTURE_CUSTOMER_REF;
export const FAKE_CARD_BRAND = 'visa';
export const FAKE_CURRENCY = 'BRL';
export const FAKE_AMOUNT_CENTS = NORMAL_AMOUNT_CENTS;
export const FAKE_REQUESTED_AT = FIXTURE_NOW;
/** Latency of the stored assessments built here: `completedAt - requestedAt`. */
export const FAKE_STORED_LATENCY_MS = 42;
/** Values that differ from the stored assessment, for conflicting repeats. */
export const FAKE_CONFLICTING_MERCHANT_ID = '00000000-0000-4000-8000-0000000000b2';
export const FAKE_CONFLICTING_AMOUNT_CENTS = FAKE_AMOUNT_CENTS + 1;
export const FAKE_CONFLICTING_CURRENCY = 'USD';

/**
 * Ids a fresh `SequentialIdGen` hands out on a new payment intent: the assessment
 * first, then the outbox event.
 */
export const FIRST_GENERATED_ASSESSMENT_ID = 'gen_001';
export const FIRST_GENERATED_EVENT_ID = 'gen_002';

/** A scenario outside `TestScenario`, as an unvalidated caller could send it. */
export const UNKNOWN_TEST_SCENARIO = 'approve' as TestScenario;

/** A valid scoring request for a first-time card; override what the test is about. */
export function scoreRiskInput(overrides: Partial<ScoreRiskInput> = {}): ScoreRiskInput {
  return {
    paymentIntentId: FAKE_PAYMENT_INTENT_ID,
    merchantId: FAKE_MERCHANT_ID,
    amountCents: FAKE_AMOUNT_CENTS,
    currency: FAKE_CURRENCY,
    cardFingerprint: FAKE_CARD_FINGERPRINT,
    cardBrand: FAKE_CARD_BRAND,
    customerRef: FAKE_CUSTOMER_REF,
    ...overrides,
  };
}

interface StoredVerdict {
  decision: Decision;
  score: number;
  reasons: FraudReason[];
}

/**
 * An assessment for `FAKE_PAYMENT_INTENT_ID`, completed with the given verdict
 * `FAKE_STORED_LATENCY_MS` after it was requested.
 */
export function completedAssessment(verdict: StoredVerdict): RiskAssessment {
  const assessment = pendingAssessment();
  const completedAt = new Date(FAKE_REQUESTED_AT.getTime() + FAKE_STORED_LATENCY_MS);
  assessment.complete(verdict.decision, verdict.score, verdict.reasons, completedAt);
  return assessment;
}

/** An assessment for `FAKE_PAYMENT_INTENT_ID` that never got a decision. */
export function pendingAssessment(): RiskAssessment {
  return RiskAssessment.create({
    id: FAKE_STORED_ASSESSMENT_ID,
    paymentIntentId: FAKE_PAYMENT_INTENT_ID,
    merchantId: FAKE_MERCHANT_ID,
    accountRef: FAKE_CARD_FINGERPRINT,
    amountCents: FAKE_AMOUNT_CENTS,
    currency: FAKE_CURRENCY,
    requestedAt: FAKE_REQUESTED_AT,
  });
}

/**
 * In-memory `ScoredAssessmentStore` that honors the port contract: one assessment per
 * payment intent, a duplicate throws `DuplicateAssessmentError` and writes nothing.
 */
export class InMemoryScoredAssessmentStore implements ScoredAssessmentStore {
  /** Every successful `saveScored`, in order. */
  readonly saved: ScoredAssessment[] = [];
  /** Every payment intent id passed to `findByPaymentIntentId`, in order. */
  readonly lookups: string[] = [];
  private readonly byPaymentIntent = new Map<string, RiskAssessmentSnapshot>();
  private concurrentWinner: RiskAssessment | null = null;
  private nextSaveError: Error | null = null;

  /** Stores an assessment as if an earlier request had scored it. */
  seed(assessment: RiskAssessment): void {
    this.byPaymentIntent.set(assessment.paymentIntentId, assessment.toSnapshot());
  }

  /** Makes the next `saveScored` lose the insert race to `winner`. */
  loseNextSaveTo(winner: RiskAssessment): void {
    this.concurrentWinner = winner;
  }

  /** Makes the next `saveScored` fail with `error` and write nothing. */
  failNextSave(error: Error): void {
    this.nextSaveError = error;
  }

  async findByPaymentIntentId(paymentIntentId: string): Promise<RiskAssessment | null> {
    this.lookups.push(paymentIntentId);
    const snapshot = this.byPaymentIntent.get(paymentIntentId);
    return snapshot ? RiskAssessment.rehydrate(snapshot) : null;
  }

  async saveScored(scored: ScoredAssessment): Promise<void> {
    if (this.concurrentWinner) {
      this.seed(this.concurrentWinner);
      this.concurrentWinner = null;
    }
    if (this.nextSaveError) {
      const error = this.nextSaveError;
      this.nextSaveError = null;
      throw error;
    }
    const { paymentIntentId } = scored.assessment;
    if (this.byPaymentIntent.has(paymentIntentId)) {
      throw new DuplicateAssessmentError(`assessment exists for payment intent ${paymentIntentId}`);
    }
    this.byPaymentIntent.set(paymentIntentId, scored.assessment.toSnapshot());
    this.saved.push(scored);
  }
}

export const FAILING_READER_ERROR_MESSAGE = 'feature store unavailable';

/** Reader whose every load fails, as when the feature store is unreachable. */
export class FailingFeatureSnapshotReader implements FeatureSnapshotReader {
  async load(): Promise<FeatureSnapshot> {
    throw new Error(FAILING_READER_ERROR_MESSAGE);
  }
}
