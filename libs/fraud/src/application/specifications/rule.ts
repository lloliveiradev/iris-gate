import { FraudReasonCode } from '../../domain/reason';
import { FeatureSnapshot } from '../feature-snapshot';

/** Everything a rule may read. Rules do no I/O: history arrives through `features`. */
export interface RuleContext {
  features: FeatureSnapshot;
  amountCents: number;
  now: Date;
}

export interface RuleOutcome {
  matched: boolean;
  baseScore: number;
  description: string;
}

export interface FraudSpecification {
  readonly code: FraudReasonCode;
  evaluate(ctx: RuleContext): RuleOutcome;
}

export const NO_MATCH: RuleOutcome = Object.freeze({
  matched: false,
  baseScore: 0,
  description: '',
});
