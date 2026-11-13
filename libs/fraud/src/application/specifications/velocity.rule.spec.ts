import { FIXTURE_NOW, firstTimeCardFeatures, NORMAL_AMOUNT_CENTS } from '../fixtures';
import { RuleContext } from './rule';
import { VelocitySpec } from './velocity.rule';

const MAX_ATTEMPTS = 5;

const ctx = (attempts24h: number): RuleContext => ({
  features: firstTimeCardFeatures({ attempts24h }),
  amountCents: NORMAL_AMOUNT_CENTS,
  now: FIXTURE_NOW,
});

describe('VelocitySpec', () => {
  const rule = new VelocitySpec({ maxAttempts24h: MAX_ATTEMPTS });

  it('uses the VELOCITY code', () => {
    expect(rule.code).toBe('VELOCITY');
  });

  it.each([
    ['no attempts', 0, false, 0],
    ['at the limit', MAX_ATTEMPTS, false, 0],
    ['one over', MAX_ATTEMPTS + 1, true, 70],
    ['three over', MAX_ATTEMPTS + 3, true, 90],
    ['four over reaches the cap', MAX_ATTEMPTS + 4, true, 100],
    ['far over stays capped', MAX_ATTEMPTS + 50, true, 100],
  ])('%s', (_label, attempts, matched, baseScore) => {
    expect(rule.evaluate(ctx(attempts))).toMatchObject({ matched, baseScore });
  });

  it('describes the count and the limit', () => {
    expect(rule.evaluate(ctx(MAX_ATTEMPTS + 1)).description).toBe('6 attempts in last 24h (max 5)');
  });
});
