import { FraudReasonCode } from '../../domain/reason';
import { FraudSpecification, RuleContext, RuleOutcome } from './rule';

export interface UnusualAmountParams {
  historyCount: number;
  thresholdPercent: number;
}

export class UnusualAmountSpec implements FraudSpecification {
  readonly code = FraudReasonCode.UNUSUAL_AMOUNT;

  constructor(private readonly params: UnusualAmountParams) {}

  async evaluate(ctx: RuleContext): Promise<RuleOutcome> {
    const prev = await ctx.history.lastNByAccount(ctx.account.ref, this.params.historyCount);
    if (prev.length === 0) {
      return { matched: false, baseScore: 0, description: '' };
    }

    const total = prev.reduce((sum, inv) => sum + inv.amountCents, 0);
    const avg = total / prev.length;
    const threshold = avg + (avg * this.params.thresholdPercent) / 100 + avg;

    if (ctx.amountCents <= threshold) {
      return { matched: false, baseScore: 0, description: '' };
    }

    const ratio = ctx.amountCents / Math.max(threshold, 1);
    const baseScore = Math.min(100, 50 + (ratio - 1) * 50);

    return {
      matched: true,
      baseScore,
      description: `amount ${ctx.amountCents} exceeds threshold ${threshold.toFixed(0)} (avg=${avg.toFixed(0)})`,
    };
  }
}
