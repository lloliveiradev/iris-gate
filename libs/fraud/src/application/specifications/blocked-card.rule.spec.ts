import { FIXTURE_NOW, firstTimeCardFeatures, NORMAL_AMOUNT_CENTS } from '../fixtures';
import { BlockedCardSpec } from './blocked-card.rule';
import { RuleContext } from './rule';

const ctx = (blocked: boolean): RuleContext => ({
  features: firstTimeCardFeatures({ blocked }),
  amountCents: NORMAL_AMOUNT_CENTS,
  now: FIXTURE_NOW,
});

describe('BlockedCardSpec', () => {
  const rule = new BlockedCardSpec();

  it('uses the BLOCKED_CARD code', () => {
    expect(rule.code).toBe('BLOCKED_CARD');
  });

  it.each([
    ['blocked card', true, { matched: true, baseScore: 100 }],
    ['clean card', false, { matched: false, baseScore: 0 }],
  ])('%s', (_label, blocked, expected) => {
    expect(rule.evaluate(ctx(blocked))).toMatchObject(expected);
  });

  it('does not put the card fingerprint in the description', () => {
    const out = rule.evaluate(ctx(true));
    expect(out.description).toBe('card is blocked');
  });
});
