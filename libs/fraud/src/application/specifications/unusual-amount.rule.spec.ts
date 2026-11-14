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

  // avg = NORMAL_AMOUNT_CENTS (1000); 50% means threshold = 1.5 * 1000 = 1500.
  it.each([
    ['the average itself', 1_000, false],
    ['exactly 1.5x the average (strictly-above comparison)', 1_500, false],
    ['one cent above 1.5x the average', 1_501, true],
    ['the old 2.5x threshold', 2_500, true],
  ])('%s', (_label, amountCents, matched) => {
    expect(rule.evaluate(ctx(NORMAL_AMOUNT_CENTS, amountCents)).matched).toBe(matched);
  });

  it('scores 50 + (ratio - 1) * 50 against the threshold, capped at 100', () => {
    expect(rule.evaluate(ctx(NORMAL_AMOUNT_CENTS, 3_000))).toEqual({
      matched: true,
      baseScore: 100,
      description: 'amount 3000 exceeds threshold 1500 (avg=1000)',
    });
    expect(rule.evaluate(ctx(NORMAL_AMOUNT_CENTS, 2_250)).baseScore).toBe(75);
  });

  it('treats thresholdPercent 0 as "any amount above the average"', () => {
    const strict = new UnusualAmountSpec({ thresholdPercent: 0 });
    expect(strict.evaluate(ctx(NORMAL_AMOUNT_CENTS, NORMAL_AMOUNT_CENTS)).matched).toBe(false);
    expect(strict.evaluate(ctx(NORMAL_AMOUNT_CENTS, NORMAL_AMOUNT_CENTS + 1)).matched).toBe(true);
  });

  it('compares amounts above the 32-bit range exactly', () => {
    // avg = 2^32 cents, so the threshold 1.5 * avg = 3 * 2^31 is an exact integer.
    const avg = 2 * ABOVE_INT32_AMOUNT_CENTS;
    const threshold = 3 * ABOVE_INT32_AMOUNT_CENTS;
    expect(rule.evaluate(ctx(avg, threshold)).matched).toBe(false);
    expect(rule.evaluate(ctx(avg, threshold + 1)).matched).toBe(true);
  });

  it('stays exact near Number.MAX_SAFE_INTEGER, where amount * 100 is not a safe number', () => {
    // Pick avg so that the threshold 1.5 * avg is exactly MAX_SAFE_INTEGER - 1.
    const avg = ((Number.MAX_SAFE_INTEGER - 1) / 3) * 2;
    const threshold = (avg / 2) * 3;
    expect(rule.evaluate(ctx(avg, threshold)).matched).toBe(false);
    expect(rule.evaluate(ctx(avg, threshold + 1)).matched).toBe(true);
  });
});
