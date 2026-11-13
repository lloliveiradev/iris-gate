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
