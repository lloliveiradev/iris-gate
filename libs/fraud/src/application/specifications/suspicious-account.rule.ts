import { FraudReasonCode } from '../../domain/reason';
import { FraudSpecification, RuleContext, RuleOutcome } from './rule';

export class SuspiciousAccountSpec implements FraudSpecification {
  readonly code = FraudReasonCode.SUSPICIOUS_ACCOUNT;

   
  async evaluate(ctx: RuleContext): Promise<RuleOutcome> {
    if (ctx.account.isSuspicious) {
      return {
        matched: true,
        baseScore: 100,
        description: `account ${ctx.account.ref} flagged as suspicious`,
      };
    }
    return { matched: false, baseScore: 0, description: '' };
  }
}
