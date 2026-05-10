import { FraudReasonCode } from '../../domain/reason';
import { FraudSpecification, RuleContext, RuleOutcome } from './rule';

export interface FrequentHighValueParams {
  maxRecentCount: number;
  windowHours: number;
}

export class FrequentHighValueSpec implements FraudSpecification {
  readonly code = FraudReasonCode.FREQUENT_HIGH_VALUE;

  constructor(private readonly params: FrequentHighValueParams) {}

  async evaluate(ctx: RuleContext): Promise<RuleOutcome> {
    const since = new Date(ctx.now.getTime() - this.params.windowHours * 60 * 60 * 1000);
    const recent = await ctx.history.recentByAccount(ctx.account.ref, since);

    if (recent.length <= this.params.maxRecentCount) {
      return { matched: false, baseScore: 0, description: '' };
    }

    const overage = recent.length - this.params.maxRecentCount;
    const baseScore = Math.min(100, 60 + overage * 10);

    return {
      matched: true,
      baseScore,
      description: `${recent.length} payments in last ${this.params.windowHours}h (max ${this.params.maxRecentCount})`,
    };
  }
}
