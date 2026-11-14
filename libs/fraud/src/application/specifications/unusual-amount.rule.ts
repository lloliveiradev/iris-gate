import { FraudReasonCode } from '../../domain/reason';
import { FraudSpecification, NO_MATCH, RuleContext, RuleOutcome } from './rule';

const PERCENT = 100;
const PERCENT_BIG = BigInt(PERCENT);
const MATCH_BASE_SCORE = 50;
const MAX_BASE_SCORE = 100;

export interface UnusualAmountParams {
  /** Whole percent above the average that is still usual: 50 means up to 1.5x. */
  thresholdPercent: number;
}

/**
 * Matches an amount strictly above `avg * (1 + thresholdPercent / 100)`, where `avg`
 * is the card's average captured amount. An amount exactly on the threshold does
 * not match. A card with no captured history never matches.
 *
 * The comparison runs in `bigint` (`amount * 100 > avg * (100 + thresholdPercent)`),
 * so it stays exact for any safe-integer amount, including amounts above 2^31.
 * `thresholdPercent` must therefore be an integer; the rule registry enforces it.
 */
export class UnusualAmountSpec implements FraudSpecification {
  readonly code = FraudReasonCode.UNUSUAL_AMOUNT;

  constructor(private readonly params: UnusualAmountParams) {}

  evaluate(ctx: RuleContext): RuleOutcome {
    const avg = ctx.features.avgCapturedCents;
    if (avg === null) return NO_MATCH;

    const scaledAmount = BigInt(ctx.amountCents) * PERCENT_BIG;
    const scaledThreshold = BigInt(avg) * BigInt(PERCENT + this.params.thresholdPercent);
    if (scaledAmount <= scaledThreshold) return NO_MATCH;

    // The comparison above is exact; the score only needs an approximate ratio.
    const ratio = Number(scaledAmount) / Math.max(Number(scaledThreshold), 1);
    const baseScore = Math.min(MAX_BASE_SCORE, MATCH_BASE_SCORE + (ratio - 1) * MATCH_BASE_SCORE);
    const threshold = scaledThreshold / PERCENT_BIG;

    return {
      matched: true,
      baseScore,
      description: `amount ${ctx.amountCents} exceeds threshold ${threshold} (avg=${avg})`,
    };
  }
}
