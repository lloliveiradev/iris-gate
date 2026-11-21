import { DEFAULT_INTERNAL_HOST, DEFAULT_INTERNAL_PORT, loadRiskConfig } from './risk.config';
import { TEST_SERVICE_TOKEN } from '../modules/risk/risk.test-fixtures';

describe('loadRiskConfig', () => {
  it.each([
    ['missing', {}],
    ['empty string', { FRAUD_SERVICE_TOKEN: '' }],
    ['whitespace only', { FRAUD_SERVICE_TOKEN: '   ' }],
  ])('fails fast when FRAUD_SERVICE_TOKEN is %s', (_label, env) => {
    expect(() => loadRiskConfig(env)).toThrow('FRAUD_SERVICE_TOKEN is not set');
  });

  it('defaults internalHost to 127.0.0.1, internalPort to 13002, and testHooksEnabled to false', () => {
    expect(loadRiskConfig({ FRAUD_SERVICE_TOKEN: TEST_SERVICE_TOKEN })).toEqual({
      internalHost: DEFAULT_INTERNAL_HOST,
      internalPort: DEFAULT_INTERNAL_PORT,
      serviceToken: TEST_SERVICE_TOKEN,
      testHooksEnabled: false,
    });
  });

  it.each([
    ['true', true],
    ['false', false],
    ['1', false],
    ['TRUE', false],
    ['', false],
  ])('parses FRAUD_TEST_HOOKS=%p as %p', (rawValue, expected) => {
    const cfg = loadRiskConfig({
      FRAUD_SERVICE_TOKEN: TEST_SERVICE_TOKEN,
      FRAUD_TEST_HOOKS: rawValue,
    });
    expect(cfg.testHooksEnabled).toBe(expected);
  });

  it('refuses FRAUD_TEST_HOOKS=true when NODE_ENV=production', () => {
    expect(() =>
      loadRiskConfig({
        FRAUD_SERVICE_TOKEN: TEST_SERVICE_TOKEN,
        NODE_ENV: 'production',
        FRAUD_TEST_HOOKS: 'true',
      }),
    ).toThrow('FRAUD_TEST_HOOKS=true is refused when NODE_ENV=production');
  });

  it.each([
    ['unset', undefined],
    ['false', 'false'],
    ['TRUE', 'TRUE'],
    ['1', '1'],
    ['empty', ''],
  ])('allows FRAUD_TEST_HOOKS %s when NODE_ENV=production', (_label, rawValue) => {
    const cfg = loadRiskConfig({
      FRAUD_SERVICE_TOKEN: TEST_SERVICE_TOKEN,
      NODE_ENV: 'production',
      ...(rawValue === undefined ? {} : { FRAUD_TEST_HOOKS: rawValue }),
    });
    expect(cfg.testHooksEnabled).toBe(false);
  });

  it.each(['development', 'test', undefined])(
    'allows FRAUD_TEST_HOOKS=true when NODE_ENV is %p',
    (nodeEnv) => {
      const cfg = loadRiskConfig({
        FRAUD_SERVICE_TOKEN: TEST_SERVICE_TOKEN,
        ...(nodeEnv === undefined ? {} : { NODE_ENV: nodeEnv }),
        FRAUD_TEST_HOOKS: 'true',
      });
      expect(cfg.testHooksEnabled).toBe(true);
    },
  );

  it('reads custom ANTI_FRAUD_INTERNAL_PORT and ANTI_FRAUD_INTERNAL_HOST', () => {
    const cfg = loadRiskConfig({
      FRAUD_SERVICE_TOKEN: TEST_SERVICE_TOKEN,
      ANTI_FRAUD_INTERNAL_PORT: '14002',
      ANTI_FRAUD_INTERNAL_HOST: '0.0.0.0',
    });
    expect(cfg.internalPort).toBe(14002);
    expect(cfg.internalHost).toBe('0.0.0.0');
  });

  it.each(['0', '-1', '65536', '13002.5', 'not-a-port'])(
    'rejects invalid ANTI_FRAUD_INTERNAL_PORT %p',
    (rawPort) => {
      expect(() =>
        loadRiskConfig({
          FRAUD_SERVICE_TOKEN: TEST_SERVICE_TOKEN,
          ANTI_FRAUD_INTERNAL_PORT: rawPort,
        }),
      ).toThrow('ANTI_FRAUD_INTERNAL_PORT');
    },
  );
});
