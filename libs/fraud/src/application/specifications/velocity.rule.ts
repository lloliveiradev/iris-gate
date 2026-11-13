import { FraudReasonCode } from '../../domain/reason';
import { overageScore } from './overage-score';
import { FraudSpecification, NO_MATCH, RuleContext, RuleOutcome } from './rule';

export interface VelocityParams {
  maxAttempts24h: number;
}

/**
 * Matches a card whose payment attempts in the last 24 hours, counting the current
 * attempt being scored (`attempts24h + 1`), exceed `maxAttempts24h`.
 */
export class VelocitySpec implements FraudSpecification {
  readonly code = FraudReasonCode.VELOCITY;

  constructor(private readonly params: VelocityParams) {}

  evaluate(ctx: RuleContext): RuleOutcome {
    const attemptsWithCurrent = ctx.features.attempts24h + 1;
    const overage = attemptsWithCurrent - this.params.maxAttempts24h;
    if (overage <= 0) return NO_MATCH;

    return {
      matched: true,
      baseScore: overageScore(overage),
      description: `${attemptsWithCurrent} attempts in last 24h (max ${this.params.maxAttempts24h})`,
    };
  }
}
