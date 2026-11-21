export const DEFAULT_INTERNAL_HOST = '127.0.0.1';
export const DEFAULT_INTERNAL_PORT = 13002;

const MIN_TCP_PORT = 1;
const MAX_TCP_PORT = 65535;
const PRODUCTION_ENV = 'production';
const ENABLED_TEST_HOOKS = 'true';

export interface RiskConfig {
  internalHost: string;
  internalPort: number;
  serviceToken: string;
  testHooksEnabled: boolean;
}

/**
 * Parses a TCP listen port with the same bounds for every listener.
 * A missing or blank value uses `defaultPort`. Any other value must be an
 * integer in 1..65535.
 */
export function parseTcpPort(raw: string | undefined, name: string, defaultPort: number): number {
  const port = raw === undefined || raw.trim() === '' ? defaultPort : Number(raw);
  if (!Number.isSafeInteger(port) || port < MIN_TCP_PORT || port > MAX_TCP_PORT) {
    throw new Error(
      `${name} must be an integer in ${MIN_TCP_PORT}..${MAX_TCP_PORT}, got ${String(raw)}`,
    );
  }
  return port;
}

/**
 * Reads the configuration of the internal risk-scoring listener.
 * Fails fast when `FRAUD_SERVICE_TOKEN` is missing or blank, when
 * `FRAUD_TEST_HOOKS=true` under `NODE_ENV=production`, or when
 * `ANTI_FRAUD_INTERNAL_PORT` is not a valid TCP port.
 */
export function loadRiskConfig(env: NodeJS.ProcessEnv = process.env): RiskConfig {
  const serviceToken = env.FRAUD_SERVICE_TOKEN?.trim() ?? '';
  if (serviceToken.length === 0) {
    throw new Error('FRAUD_SERVICE_TOKEN is not set');
  }

  const testHooksEnabled = env.FRAUD_TEST_HOOKS === ENABLED_TEST_HOOKS;
  if (env.NODE_ENV === PRODUCTION_ENV && testHooksEnabled) {
    throw new Error('FRAUD_TEST_HOOKS=true is refused when NODE_ENV=production');
  }

  const internalPort = parseTcpPort(
    env.ANTI_FRAUD_INTERNAL_PORT,
    'ANTI_FRAUD_INTERNAL_PORT',
    DEFAULT_INTERNAL_PORT,
  );

  const rawHost = env.ANTI_FRAUD_INTERNAL_HOST?.trim();
  const internalHost = rawHost && rawHost.length > 0 ? rawHost : DEFAULT_INTERNAL_HOST;

  return {
    internalHost,
    internalPort,
    serviceToken,
    testHooksEnabled,
  };
}

export const RISK_CONFIG = Symbol('RISK_CONFIG');
