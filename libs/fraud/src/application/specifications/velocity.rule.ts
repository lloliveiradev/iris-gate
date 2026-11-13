import { FraudReasonCode } from '../../domain/reason';
import { overageScore } from './overage-score';
import { FraudSpecification, NO_MATCH, RuleContext, RuleOutcome } from './rule';

export interface VelocityParams {
  maxAttempts24h: number;
}

/** Matches a card with more payment attempts in the last 24 hours than allowed. */
export class VelocitySpec implements FraudSpecification {
  readonly code = FraudReasonCode.VELOCITY;

  constructor(private readonly params: VelocityParams) {}

  evaluate(ctx: RuleContext): RuleOutcome {
    const { attempts24h } = ctx.features;
    const overage = attempts24h - this.params.maxAttempts24h;
    if (overage <= 0) return NO_MATCH;

    return {
      matched: true,
      baseScore: overageScore(overage),
      description: `${attempts24h} attempts in last 24h (max ${this.params.maxAttempts24h})`,
    };
  }
}
