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
    ['no prior attempts (attempt 1)', 0, false, 0],
    ['at the limit (attempt MAX_ATTEMPTS)', MAX_ATTEMPTS - 1, false, 0],
    ['one over (attempt MAX_ATTEMPTS + 1)', MAX_ATTEMPTS, true, 70],
    ['three over (attempt MAX_ATTEMPTS + 3)', MAX_ATTEMPTS + 2, true, 90],
    ['four over reaches the cap (attempt MAX_ATTEMPTS + 4)', MAX_ATTEMPTS + 3, true, 100],
    ['far over stays capped', MAX_ATTEMPTS + 50, true, 100],
  ])('%s', (_label, priorAttempts, matched, baseScore) => {
    expect(rule.evaluate(ctx(priorAttempts))).toMatchObject({ matched, baseScore });
  });

  it('describes the total count including current attempt and the limit', () => {
    expect(rule.evaluate(ctx(MAX_ATTEMPTS)).description).toBe('6 attempts in last 24h (max 5)');
  });
});
