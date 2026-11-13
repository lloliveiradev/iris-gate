import { Decision } from './decision';
import { FraudReason, FraudReasonCode } from './reason';
import { DecisionThresholds } from './thresholds';

/** Scenarios a test request may force. Matching is exact and case-sensitive. */
export const TestScenario = {
  BLOCKED: 'blocked',
  REVIEW: 'review',
} as const;

export type TestScenario = (typeof TestScenario)[keyof typeof TestScenario];

/** A decision forced by a test scenario, replacing the rule evaluation. */
export interface TestScenarioOverride {
  readonly decision: Decision;
  readonly score: number;
  readonly reasons: FraudReason[];
}

const FULL_WEIGHT = 1;

/**
 * Resolves the test-hook override: `blocked` forces `REJECT`, `review` forces
 * `REVIEW`, each with a single `TEST_SCENARIO` reason. The score sits on the
 * threshold of the forced decision so that `thresholds.classify(score)` agrees with it.
 * Any other value, including none, yields `null` (no override). Whether test hooks
 * are enabled at all is the caller's decision.
 */
export function resolveTestScenario(
  scenario: string | null | undefined,
  thresholds: DecisionThresholds,
): TestScenarioOverride | null {
  switch (scenario) {
    case TestScenario.BLOCKED:
      return forced(scenario, Decision.REJECT, thresholds.reject);
    case TestScenario.REVIEW:
      return forced(scenario, Decision.REVIEW, thresholds.review);
    default:
      return null;
  }
}

function forced(scenario: TestScenario, decision: Decision, score: number): TestScenarioOverride {
  return {
    decision,
    score,
    reasons: [
      {
        code: FraudReasonCode.TEST_SCENARIO,
        weight: FULL_WEIGHT,
        score,
        description: `test scenario ${scenario} forced ${decision}`,
      },
    ],
  };
}
