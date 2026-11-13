import { InvalidRuleConfigError } from '../domain/errors';
import { FraudReasonCode } from '../domain/reason';
import { RuleConfig } from './ports';
import { BlockedCardSpec } from './specifications/blocked-card.rule';
import { DeclineBurstSpec } from './specifications/decline-burst.rule';
import { FraudSpecification } from './specifications/rule';
import { UnusualAmountSpec } from './specifications/unusual-amount.rule';
import { VelocitySpec } from './specifications/velocity.rule';

export interface RegisteredRule {
  spec: FraudSpecification;
  weight: number;
}

const MIN_WEIGHT = 0;
const MAX_WEIGHT = 1;

/** Default rule set, used when no rule configuration is stored. */
export const DEFAULT_RULES: RuleConfig[] = [
  { code: FraudReasonCode.BLOCKED_CARD, weight: 1.0, enabled: true, params: {} },
  {
    code: FraudReasonCode.UNUSUAL_AMOUNT,
    weight: 0.6,
    enabled: true,
    params: { thresholdPercent: 50 },
  },
  // Placeholder default pending owner review (phase 10 plan, gap G6).
  {
    code: FraudReasonCode.VELOCITY,
    weight: 0.5,
    enabled: true,
    params: { maxAttempts24h: 5 },
  },
  // Placeholder default pending owner review (phase 10 plan, gap G6).
  {
    code: FraudReasonCode.DECLINE_BURST,
    weight: 0.7,
    enabled: true,
    params: { maxDeclines24h: 3 },
  },
];

type SpecFactory = (cfg: RuleConfig) => FraudSpecification;

// A new rule is a new entry here; codes not listed are rejected.
const SPEC_FACTORIES: ReadonlyMap<string, SpecFactory> = new Map<string, SpecFactory>([
  [FraudReasonCode.BLOCKED_CARD, () => new BlockedCardSpec()],
  [
    FraudReasonCode.UNUSUAL_AMOUNT,
    (cfg) => new UnusualAmountSpec({ thresholdPercent: nonNegativeParam(cfg, 'thresholdPercent') }),
  ],
  [
    FraudReasonCode.VELOCITY,
    (cfg) => new VelocitySpec({ maxAttempts24h: positiveIntegerParam(cfg, 'maxAttempts24h') }),
  ],
  [
    FraudReasonCode.DECLINE_BURST,
    (cfg) => new DeclineBurstSpec({ maxDeclines24h: positiveIntegerParam(cfg, 'maxDeclines24h') }),
  ],
]);

/**
 * Builds the enabled rules from configuration. Throws `InvalidRuleConfigError` on an
 * unknown code, a weight that is not a finite number in [0..1], or a missing or
 * invalid param, so a bad configuration stops startup instead of scoring loosely.
 */
export function buildRulesFromConfig(configs: RuleConfig[]): RegisteredRule[] {
  const result: RegisteredRule[] = [];
  for (const cfg of configs) {
    if (!cfg.enabled) continue;
    const factory = SPEC_FACTORIES.get(cfg.code);
    if (!factory) {
      throw new InvalidRuleConfigError(`unknown rule code: ${cfg.code}`);
    }
    result.push({ spec: factory(cfg), weight: validWeight(cfg) });
  }
  return result;
}

function validWeight(cfg: RuleConfig): number {
  const { weight } = cfg;
  if (!Number.isFinite(weight) || weight < MIN_WEIGHT || weight > MAX_WEIGHT) {
    throw new InvalidRuleConfigError(
      `rule ${cfg.code} weight out of [${MIN_WEIGHT}..${MAX_WEIGHT}]: ${weight}`,
    );
  }
  return weight;
}

function numericParam(cfg: RuleConfig, key: string): number {
  const value = cfg.params[key];
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new InvalidRuleConfigError(`rule ${cfg.code} missing numeric param ${key}`);
  }
  return value;
}

function nonNegativeParam(cfg: RuleConfig, key: string): number {
  const value = numericParam(cfg, key);
  if (value < 0) {
    throw new InvalidRuleConfigError(`rule ${cfg.code} param ${key} must be >= 0: ${value}`);
  }
  return value;
}

function positiveIntegerParam(cfg: RuleConfig, key: string): number {
  const value = numericParam(cfg, key);
  if (!Number.isSafeInteger(value) || value <= 0) {
    throw new InvalidRuleConfigError(
      `rule ${cfg.code} param ${key} must be a positive integer: ${value}`,
    );
  }
  return value;
}
