import { Decision, mostSevere } from './decision';
import { InvalidAssessmentInputError } from './errors';
import { FraudReason, FraudReasonCode } from './reason';
import { DecisionThresholds } from './thresholds';

/** Scenarios a test request may ask for. Matching is exact and case-sensitive. */
export const TestScenario = {
  BLOCKED: 'blocked',
  REVIEW: 'review',
} as const;

export type TestScenario = (typeof TestScenario)[keyof typeof TestScenario];

/** A scored decision and the reasons behind it. */
export interface RiskVerdict {
  readonly decision: Decision;
  readonly score: number;
  readonly reasons: readonly FraudReason[];
}

const FULL_WEIGHT = 1;

/**
 * Applies a test scenario on top of the rule verdict. The scenario can only raise
 * the outcome, never lower it: the decision becomes the more severe of the rule
 * decision and the scenario's (`blocked` → `REJECT`, `review` → `REVIEW`), so
 * `review` never downgrades a `REJECT`. The score becomes the higher of the rule
 * score and the threshold of the scenario's decision, and a `TEST_SCENARIO` reason
 * is added to the rule reasons. Whether test hooks are enabled at all is the
 * caller's decision; an unknown scenario throws instead of being ignored.
 */
export function applyTestScenario(
  verdict: RiskVerdict,
  scenario: TestScenario,
  thresholds: DecisionThresholds,
): RiskVerdict {
  const required = requiredDecision(scenario);
  const floorScore = required === Decision.REJECT ? thresholds.reject : thresholds.review;
  const reason: FraudReason = {
    code: FraudReasonCode.TEST_SCENARIO,
    weight: FULL_WEIGHT,
    score: floorScore,
    description: `test scenario ${scenario} requires at least ${required}`,
  };
  return {
    decision: mostSevere(verdict.decision, required),
    score: Math.max(verdict.score, floorScore),
    reasons: [...verdict.reasons, reason],
  };
}

/**
 * Throws `InvalidAssessmentInputError` when `scenario` is not a known `TestScenario`,
 * so a caller can reject it before doing any I/O.
 */
export function assertKnownTestScenario(scenario: TestScenario): void {
  requiredDecision(scenario);
}

function requiredDecision(scenario: TestScenario): Decision {
  switch (scenario) {
    case TestScenario.BLOCKED:
      return Decision.REJECT;
    case TestScenario.REVIEW:
      return Decision.REVIEW;
    default:
      return unknownScenario(scenario);
  }
}

function unknownScenario(scenario: never): never {
  throw new InvalidAssessmentInputError(`unknown test scenario: ${String(scenario)}`);
}
