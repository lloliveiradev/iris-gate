import { Decision } from './decision';
import { InvalidAssessmentInputError } from './errors';
import { FraudReason } from './reason';
import { applyTestScenario, RiskVerdict, TestScenario } from './test-scenario';
import { DecisionThresholds } from './thresholds';

const thresholds = DecisionThresholds.defaults();

const BLOCKED_CARD_REASON: FraudReason = {
  code: 'BLOCKED_CARD',
  weight: 1,
  score: 100,
  description: 'card is blocked',
};
const VELOCITY_REASON: FraudReason = {
  code: 'VELOCITY',
  weight: 0.5,
  score: 70,
  description: '6 attempts in last 24h (max 5)',
};

const APPROVED: RiskVerdict = { decision: Decision.APPROVE, score: 0, reasons: [] };
const REVIEWED: RiskVerdict = { decision: Decision.REVIEW, score: 35, reasons: [VELOCITY_REASON] };
const REJECTED: RiskVerdict = {
  decision: Decision.REJECT,
  score: 100,
  reasons: [BLOCKED_CARD_REASON],
};

const testScenarioReason = (scenario: TestScenario, decision: Decision, score: number) => ({
  code: 'TEST_SCENARIO',
  weight: 1,
  score,
  description: `test scenario ${scenario} requires at least ${decision}`,
});

describe('applyTestScenario', () => {
  it.each<[string, RiskVerdict, TestScenario, Decision, number]>([
    ['blocked raises APPROVE to REJECT', APPROVED, 'blocked', Decision.REJECT, 70],
    ['blocked raises REVIEW to REJECT', REVIEWED, 'blocked', Decision.REJECT, 70],
    ['blocked keeps REJECT', REJECTED, 'blocked', Decision.REJECT, 100],
    ['review raises APPROVE to REVIEW', APPROVED, 'review', Decision.REVIEW, 30],
    ['review keeps REVIEW and the higher rule score', REVIEWED, 'review', Decision.REVIEW, 35],
    ['review never downgrades REJECT', REJECTED, 'review', Decision.REJECT, 100],
  ])('%s', (_label, verdict, scenario, decision, score) => {
    const out = applyTestScenario(verdict, scenario, thresholds);
    expect(out.decision).toBe(decision);
    expect(out.score).toBe(score);
    expect(thresholds.classify(out.score)).toBe(out.decision);
  });

  it('adds TEST_SCENARIO to the rule reasons (union, rule reasons first)', () => {
    expect(applyTestScenario(REJECTED, 'review', thresholds).reasons).toEqual([
      BLOCKED_CARD_REASON,
      testScenarioReason('review', Decision.REVIEW, 30),
    ]);
    expect(applyTestScenario(APPROVED, 'blocked', thresholds).reasons).toEqual([
      testScenarioReason('blocked', Decision.REJECT, 70),
    ]);
  });

  it('does not mutate the rule verdict', () => {
    const reasons = [VELOCITY_REASON];
    const verdict: RiskVerdict = { decision: Decision.REVIEW, score: 35, reasons };
    applyTestScenario(verdict, 'blocked', thresholds);
    expect(reasons).toEqual([VELOCITY_REASON]);
  });

  it('uses the configured thresholds as the score floor', () => {
    const custom = DecisionThresholds.of(40, 90);
    expect(applyTestScenario(APPROVED, 'review', custom).score).toBe(40);
    expect(applyTestScenario(APPROVED, 'blocked', custom).score).toBe(90);
  });

  it.each(['approve', 'BLOCKED', ' review', ''])(
    'throws on the unknown scenario %p instead of ignoring it',
    (scenario) => {
      // Simulates an unvalidated value reaching the domain from an untyped caller.
      const unknown = scenario as TestScenario;
      expect(() => applyTestScenario(APPROVED, unknown, thresholds)).toThrow(
        InvalidAssessmentInputError,
      );
    },
  );
});
