import { FraudReasonCode } from '../domain/reason';
import { RuleConfig } from './ports';
import { FrequentHighValueSpec } from './specifications/frequent-high-value.rule';
import { FraudSpecification } from './specifications/rule';
import { SuspiciousAccountSpec } from './specifications/suspicious-account.rule';
import { UnusualAmountSpec } from './specifications/unusual-amount.rule';

export interface RegisteredRule {
  spec: FraudSpecification;
  weight: number;
}

const REQUIRED_PARAM = (cfg: RuleConfig, key: string): number => {
  const v = cfg.params[key];
  if (typeof v !== 'number' || !Number.isFinite(v)) {
    throw new Error(`rule ${cfg.code} missing numeric param ${key}`);
  }
  return v;
};

export const DEFAULT_RULES: RuleConfig[] = [
  { code: FraudReasonCode.SUSPICIOUS_ACCOUNT, weight: 1.0, enabled: true, params: {} },
  {
    code: FraudReasonCode.UNUSUAL_AMOUNT,
    weight: 0.6,
    enabled: true,
    params: { historyCount: 10, thresholdPercent: 50 },
  },
  {
    code: FraudReasonCode.FREQUENT_HIGH_VALUE,
    weight: 0.5,
    enabled: true,
    params: { maxRecentCount: 5, windowHours: 1 },
  },
];

export function buildRulesFromConfig(configs: RuleConfig[]): RegisteredRule[] {
  const result: RegisteredRule[] = [];
  for (const cfg of configs) {
    if (!cfg.enabled) continue;
    if (cfg.weight < 0 || cfg.weight > 1) {
      throw new Error(`rule ${cfg.code} weight out of [0..1]: ${cfg.weight}`);
    }
    switch (cfg.code) {
      case FraudReasonCode.SUSPICIOUS_ACCOUNT:
        result.push({ spec: new SuspiciousAccountSpec(), weight: cfg.weight });
        break;
      case FraudReasonCode.UNUSUAL_AMOUNT:
        result.push({
          spec: new UnusualAmountSpec({
            historyCount: REQUIRED_PARAM(cfg, 'historyCount'),
            thresholdPercent: REQUIRED_PARAM(cfg, 'thresholdPercent'),
          }),
          weight: cfg.weight,
        });
        break;
      case FraudReasonCode.FREQUENT_HIGH_VALUE:
        result.push({
          spec: new FrequentHighValueSpec({
            maxRecentCount: REQUIRED_PARAM(cfg, 'maxRecentCount'),
            windowHours: REQUIRED_PARAM(cfg, 'windowHours'),
          }),
          weight: cfg.weight,
        });
        break;
      default:
        throw new Error(`unknown rule code: ${cfg.code}`);
    }
  }
  return result;
}
