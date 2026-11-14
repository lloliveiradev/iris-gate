import { RiskAssessment } from '../domain/risk-assessment';
import { FeatureSnapshot } from './feature-snapshot';
import { OutboxEventInput } from './ports';

/** The `attempted` payment outcome recorded when a payment intent is scored. */
export interface AttemptedOutcome {
  readonly paymentIntentId: string;
  readonly merchantId: string;
  readonly cardFingerprint: string;
  readonly cardBrand: string;
  readonly customerRef: string | null;
  readonly amountCents: number;
  readonly currency: string;
  readonly occurredAt: Date;
}

/**
 * Everything one scoring writes. `assessment` is completed; its `requestedAt` and
 * `completedAt` bound the scoring, so `latency_ms` is their difference. `features`
 * is the snapshot the rules saw, stored so the decision can be explained later.
 */
export interface ScoredAssessment {
  readonly assessment: RiskAssessment;
  readonly features: FeatureSnapshot;
  readonly attempt: AttemptedOutcome;
  readonly outboxEvent: OutboxEventInput;
}

/** Persistence port of the synchronous scoring path. */
export interface ScoredAssessmentStore {
  /** The assessment stored for the payment intent, or `null` if it was never scored. */
  findByPaymentIntentId(paymentIntentId: string): Promise<RiskAssessment | null>;

  /**
   * Stores the assessment with its feature snapshot, the `attempted` outcome and the
   * outbox record in ONE transaction: all of them or none. Throws
   * `DuplicateAssessmentError` when an assessment for the same payment intent already
   * exists (unique `payment_intent_id`), and writes nothing in that case.
   */
  saveScored(scored: ScoredAssessment): Promise<void>;
}
