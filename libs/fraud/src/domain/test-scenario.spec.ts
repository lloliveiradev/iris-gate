import { Decision } from './decision';
import { resolveTestScenario } from './test-scenario';
import { DecisionThresholds } from './thresholds';

const thresholds = DecisionThresholds.defaults();

describe('resolveTestScenario', () => {
  it.each([
    ['blocked', Decision.REJECT, 70],
    ['review', Decision.REVIEW, 30],
  ])('forces %s to %s with one TEST_SCENARIO reason', (scenario, decision, score) => {
    const override = resolveTestScenario(scenario, thresholds);
    expect(override).toEqual({
      decision,
      score,
      reasons: [
        {
          code: 'TEST_SCENARIO',
          weight: 1,
          score,
          description: `test scenario ${scenario} forced ${decision}`,
        },
      ],
    });
    expect(override && thresholds.classify(override.score)).toBe(decision);
  });

  it.each([undefined, null, '', 'approve', 'BLOCKED', ' review', 'unknown'])(
    'gives no override for %p',
    (scenario) => {
      expect(resolveTestScenario(scenario, thresholds)).toBeNull();
    },
  );
});
