export class FraudDomainError extends Error {
  constructor(message: string) {
    super(message);
    this.name = new.target.name;
  }
}

export class InvalidScoreError extends FraudDomainError {}
export class InvalidThresholdsError extends FraudDomainError {}
export class AssessmentAlreadyCompletedError extends FraudDomainError {}
export class InvalidAssessmentInputError extends FraudDomainError {}
export class InvalidFeatureSnapshotError extends FraudDomainError {}
export class InvalidRuleConfigError extends FraudDomainError {}

/**
 * An assessment already exists for the payment intent. Store adapters throw it when
 * a concurrent request won the unique `payment_intent_id` insert.
 */
export class DuplicateAssessmentError extends FraudDomainError {}

/** A stored assessment has no decision, so it cannot answer a repeated request. */
export class IncompleteAssessmentError extends FraudDomainError {}

/**
 * A request reuses the `payment_intent_id` of a stored assessment but differs in
 * `merchant_id`, `amount_cents`, `currency` or `card_fingerprint`, so the stored
 * answer does not apply.
 */
export class AssessmentConflictError extends FraudDomainError {}
