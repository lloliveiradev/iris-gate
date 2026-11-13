import { FraudReasonCode } from '../../domain/reason';
import { FraudSpecification, NO_MATCH, RuleContext, RuleOutcome } from './rule';

const PERCENT = 100;
const MATCH_BASE_SCORE = 50;
const MAX_BASE_SCORE = 100;

export interface UnusualAmountParams {
  thresholdPercent: number;
}

/**
 * Matches an amount far above the card's average captured amount. A card with no
 * captured history never matches.
 */
export class UnusualAmountSpec implements FraudSpecification {
  readonly code = FraudReasonCode.UNUSUAL_AMOUNT;

  constructor(private readonly params: UnusualAmountParams) {}

  evaluate(ctx: RuleContext): RuleOutcome {
    const avg = ctx.features.avgCapturedCents;
    if (avg === null) return NO_MATCH;

    // Kept as it was before phase 10: at 50% this is 2.5x the average, not 1.5x.
    // Flagged for owner review; do not change without a decision.
    const threshold = avg + (avg * this.params.thresholdPercent) / PERCENT + avg;
    if (ctx.amountCents <= threshold) return NO_MATCH;

    const ratio = ctx.amountCents / Math.max(threshold, 1);
    const baseScore = Math.min(MAX_BASE_SCORE, MATCH_BASE_SCORE + (ratio - 1) * MATCH_BASE_SCORE);

    return {
      matched: true,
      baseScore,
      description: `amount ${ctx.amountCents} exceeds threshold ${threshold.toFixed(0)} (avg=${avg})`,
    };
  }
}
