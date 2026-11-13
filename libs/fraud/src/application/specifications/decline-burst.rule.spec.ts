import { FIXTURE_NOW, firstTimeCardFeatures, NORMAL_AMOUNT_CENTS } from '../fixtures';
import { DeclineBurstSpec } from './decline-burst.rule';
import { RuleContext } from './rule';

const MAX_DECLINES = 3;

const ctx = (declines24h: number): RuleContext => ({
  features: firstTimeCardFeatures({ declines24h }),
  amountCents: NORMAL_AMOUNT_CENTS,
  now: FIXTURE_NOW,
});

describe('DeclineBurstSpec', () => {
  const rule = new DeclineBurstSpec({ maxDeclines24h: MAX_DECLINES });

  it('uses the DECLINE_BURST code', () => {
    expect(rule.code).toBe('DECLINE_BURST');
  });

  it.each([
    ['no declines', 0, false, 0],
    ['at the limit', MAX_DECLINES, false, 0],
    ['one over', MAX_DECLINES + 1, true, 70],
    ['two over', MAX_DECLINES + 2, true, 80],
    ['far over stays capped', MAX_DECLINES + 20, true, 100],
  ])('%s', (_label, declines, matched, baseScore) => {
    expect(rule.evaluate(ctx(declines))).toMatchObject({ matched, baseScore });
  });

  it('describes the count and the limit', () => {
    expect(rule.evaluate(ctx(MAX_DECLINES + 1)).description).toBe('4 declines in last 24h (max 3)');
  });
});
