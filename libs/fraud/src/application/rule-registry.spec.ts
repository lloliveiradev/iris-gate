import { InvalidRuleConfigError } from '../domain/errors';
import { FraudReasonCode } from '../domain/reason';
import { RuleConfig } from './ports';
import { buildRulesFromConfig, DEFAULT_RULES } from './rule-registry';
import { BlockedCardSpec } from './specifications/blocked-card.rule';
import { DeclineBurstSpec } from './specifications/decline-burst.rule';
import { UnusualAmountSpec } from './specifications/unusual-amount.rule';
import { VelocitySpec } from './specifications/velocity.rule';

const UNKNOWN_RULE_CODE = 'NOT_A_RULE';

const config = (overrides: Partial<RuleConfig> & Pick<RuleConfig, 'code'>): RuleConfig => ({
  weight: 0.5,
  enabled: true,
  params: {},
  ...overrides,
});

describe('DEFAULT_RULES', () => {
  it('covers the four phase 10 rules with their default weights and params', () => {
    expect(DEFAULT_RULES).toEqual([
      { code: 'BLOCKED_CARD', weight: 1.0, enabled: true, params: {} },
      { code: 'UNUSUAL_AMOUNT', weight: 0.6, enabled: true, params: { thresholdPercent: 50 } },
      { code: 'VELOCITY', weight: 0.5, enabled: true, params: { maxAttempts24h: 5 } },
      { code: 'DECLINE_BURST', weight: 0.7, enabled: true, params: { maxDeclines24h: 3 } },
    ]);
  });

  it('builds one specification per default rule', () => {
    const rules = buildRulesFromConfig(DEFAULT_RULES);
    expect(rules.map((r) => r.spec)).toEqual([
      expect.any(BlockedCardSpec),
      expect.any(UnusualAmountSpec),
      expect.any(VelocitySpec),
      expect.any(DeclineBurstSpec),
    ]);
    expect(rules.map((r) => r.weight)).toEqual([1.0, 0.6, 0.5, 0.7]);
  });
});

describe('buildRulesFromConfig', () => {
  it('skips disabled rules', () => {
    const rules = buildRulesFromConfig([
      config({ code: FraudReasonCode.BLOCKED_CARD, enabled: false }),
    ]);
    expect(rules).toHaveLength(0);
  });

  it('uses the configured limits', () => {
    const [rule] = buildRulesFromConfig([
      config({ code: FraudReasonCode.VELOCITY, params: { maxAttempts24h: 2 } }),
    ]);
    expect(rule?.spec).toEqual(new VelocitySpec({ maxAttempts24h: 2 }));
  });

  it.each([0, 1, 0.25])('accepts weight %p', (weight) => {
    const [rule] = buildRulesFromConfig([config({ code: FraudReasonCode.BLOCKED_CARD, weight })]);
    expect(rule?.weight).toBe(weight);
  });

  it.each<[string, RuleConfig, string]>([
    [
      'unknown code',
      config({ code: UNKNOWN_RULE_CODE }),
      `unknown rule code: ${UNKNOWN_RULE_CODE}`,
    ],
    ['prototype key as code', config({ code: 'constructor' }), 'unknown rule code: constructor'],
    [
      'non-rule reason code',
      config({ code: FraudReasonCode.TEST_SCENARIO }),
      'unknown rule code: TEST_SCENARIO',
    ],
    ['negative weight', config({ code: FraudReasonCode.BLOCKED_CARD, weight: -0.1 }), 'weight'],
    ['weight above 1', config({ code: FraudReasonCode.BLOCKED_CARD, weight: 1.1 }), 'weight'],
    ['NaN weight', config({ code: FraudReasonCode.BLOCKED_CARD, weight: Number.NaN }), 'weight'],
    [
      'missing thresholdPercent',
      config({ code: FraudReasonCode.UNUSUAL_AMOUNT }),
      'missing numeric param thresholdPercent',
    ],
    [
      'negative thresholdPercent',
      config({ code: FraudReasonCode.UNUSUAL_AMOUNT, params: { thresholdPercent: -1 } }),
      'thresholdPercent must be >= 0',
    ],
    [
      'infinite thresholdPercent',
      config({
        code: FraudReasonCode.UNUSUAL_AMOUNT,
        params: { thresholdPercent: Number.POSITIVE_INFINITY },
      }),
      'missing numeric param thresholdPercent',
    ],
    [
      'missing maxAttempts24h',
      config({ code: FraudReasonCode.VELOCITY }),
      'missing numeric param maxAttempts24h',
    ],
    [
      'zero maxAttempts24h',
      config({ code: FraudReasonCode.VELOCITY, params: { maxAttempts24h: 0 } }),
      'maxAttempts24h must be a positive integer',
    ],
    [
      'fractional maxAttempts24h',
      config({ code: FraudReasonCode.VELOCITY, params: { maxAttempts24h: 2.5 } }),
      'maxAttempts24h must be a positive integer',
    ],
    [
      'missing maxDeclines24h',
      config({ code: FraudReasonCode.DECLINE_BURST }),
      'missing numeric param maxDeclines24h',
    ],
    [
      'negative maxDeclines24h',
      config({ code: FraudReasonCode.DECLINE_BURST, params: { maxDeclines24h: -3 } }),
      'maxDeclines24h must be a positive integer',
    ],
  ])('rejects %s', (_label, cfg, message) => {
    expect(() => buildRulesFromConfig([cfg])).toThrow(InvalidRuleConfigError);
    expect(() => buildRulesFromConfig([cfg])).toThrow(message);
  });
});
