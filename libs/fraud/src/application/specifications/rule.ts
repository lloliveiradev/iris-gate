import { FraudReasonCode } from '../../domain/reason';
import { AccountSnapshot, InvoiceHistoryRepo } from '../ports';

export interface RuleContext {
  account: AccountSnapshot;
  amountCents: number;
  paymentIntentId: string;
  history: InvoiceHistoryRepo;
  now: Date;
}

export interface RuleOutcome {
  matched: boolean;
  baseScore: number;
  description: string;
}

export interface FraudSpecification {
  readonly code: FraudReasonCode;
  evaluate(ctx: RuleContext): Promise<RuleOutcome>;
}
