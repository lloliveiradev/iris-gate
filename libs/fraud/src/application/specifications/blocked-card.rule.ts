import { FraudReasonCode } from '../../domain/reason';
import { FraudSpecification, NO_MATCH, RuleContext, RuleOutcome } from './rule';

const BLOCKED_BASE_SCORE = 100;

/** Matches a card flagged as blocked in its profile. */
export class BlockedCardSpec implements FraudSpecification {
  readonly code = FraudReasonCode.BLOCKED_CARD;

  evaluate(ctx: RuleContext): RuleOutcome {
    if (!ctx.features.blocked) return NO_MATCH;
    return { matched: true, baseScore: BLOCKED_BASE_SCORE, description: 'card is blocked' };
  }
}
