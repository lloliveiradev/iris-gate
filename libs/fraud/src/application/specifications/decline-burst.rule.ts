import { FraudReasonCode } from '../../domain/reason';
import { overageScore } from './overage-score';
import { FraudSpecification, NO_MATCH, RuleContext, RuleOutcome } from './rule';

export interface DeclineBurstParams {
  maxDeclines24h: number;
}

/**
 * Matches a card with more prior declines in the last 24 hours (`declines24h`)
 * than `maxDeclines24h`. Because scoring happens before the acquirer is called,
 * the current attempt is not a decline and `declines24h` is compared directly.
 */
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
