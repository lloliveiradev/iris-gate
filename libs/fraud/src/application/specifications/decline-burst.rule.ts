import { FraudReasonCode } from '../../domain/reason';
import { overageScore } from './overage-score';
import { FraudSpecification, NO_MATCH, RuleContext, RuleOutcome } from './rule';

export interface DeclineBurstParams {
  maxDeclines24h: number;
}

/** Matches a card with more declines in the last 24 hours than allowed. */
export class DeclineBurstSpec implements FraudSpecification {
  readonly code = FraudReasonCode.DECLINE_BURST;

  constructor(private readonly params: DeclineBurstParams) {}

  evaluate(ctx: RuleContext): RuleOutcome {
    const { declines24h } = ctx.features;
    const overage = declines24h - this.params.maxDeclines24h;
    if (overage <= 0) return NO_MATCH;

    return {
      matched: true,
      baseScore: overageScore(overage),
      description: `${declines24h} declines in last 24h (max ${this.params.maxDeclines24h})`,
    };
  }
}
