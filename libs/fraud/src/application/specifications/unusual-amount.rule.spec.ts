import {
  ABOVE_INT32_AMOUNT_CENTS,
  FIXTURE_NOW,
  firstTimeCardFeatures,
  NORMAL_AMOUNT_CENTS,
} from '../fixtures';
import { RuleContext } from './rule';
import { UnusualAmountSpec } from './unusual-amount.rule';

const THRESHOLD_PERCENT = 50;

const ctx = (avgCapturedCents: number | null, amountCents: number): RuleContext => ({
  features: firstTimeCardFeatures({ avgCapturedCents }),
  amountCents,
  now: FIXTURE_NOW,
});

describe('UnusualAmountSpec', () => {
  const rule = new UnusualAmountSpec({ thresholdPercent: THRESHOLD_PERCENT });

  it('uses the UNUSUAL_AMOUNT code', () => {
    expect(rule.code).toBe('UNUSUAL_AMOUNT');
  });

  it('does not match without captured history', () => {
    expect(rule.evaluate(ctx(null, ABOVE_INT32_AMOUNT_CENTS)).matched).toBe(false);
  });

  // avg = NORMAL_AMOUNT_CENTS (1000); threshold = 1000 + 500 + 1000 = 2500.
  it.each([
    ['normal amount', 1_500, false],
    ['exactly at the threshold', 2_500, false],
    ['one cent above the threshold', 2_501, true],
    ['twice the threshold', 5_000, true],
  ])('%s', (_label, amountCents, matched) => {
    expect(rule.evaluate(ctx(NORMAL_AMOUNT_CENTS, amountCents)).matched).toBe(matched);
  });

  it('keeps the pre-phase-10 scoring: 50 + (ratio - 1) * 50, capped at 100', () => {
    expect(rule.evaluate(ctx(NORMAL_AMOUNT_CENTS, 5_000))).toEqual({
      matched: true,
      baseScore: 100,
      description: 'amount 5000 exceeds threshold 2500 (avg=1000)',
    });
    expect(rule.evaluate(ctx(NORMAL_AMOUNT_CENTS, 3_750)).baseScore).toBe(75);
  });

  it('compares amounts above the 32-bit range exactly', () => {
    // threshold = 2.5 * avg; the amount sits one cent above it.
    const avg = 2 * ABOVE_INT32_AMOUNT_CENTS;
    const threshold = avg * 2.5;
    expect(rule.evaluate(ctx(avg, threshold)).matched).toBe(false);
    expect(rule.evaluate(ctx(avg, threshold + 1)).matched).toBe(true);
  });
});
