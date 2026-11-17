export const DEFAULT_INTERNAL_HOST = '127.0.0.1';
export const DEFAULT_INTERNAL_PORT = 13002;

const MIN_PORT = 1;
const MAX_PORT = 65535;

export interface RiskConfig {
  internalHost: string;
  internalPort: number;
  serviceToken: string;
  testHooksEnabled: boolean;
}

/**
 * Reads the configuration of the internal risk-scoring listener.
 * Fails fast when `FRAUD_SERVICE_TOKEN` is missing or blank, or when
 * `ANTI_FRAUD_INTERNAL_PORT` is not a valid TCP port.
 */
export function loadRiskConfig(env: NodeJS.ProcessEnv = process.env): RiskConfig {
  const serviceToken = env.FRAUD_SERVICE_TOKEN?.trim() ?? '';
  if (serviceToken.length === 0) {
    throw new Error('FRAUD_SERVICE_TOKEN is not set');
  }

  const rawPort = env.ANTI_FRAUD_INTERNAL_PORT;
  const internalPort =
    rawPort === undefined || rawPort.trim() === '' ? DEFAULT_INTERNAL_PORT : Number(rawPort);
  if (!Number.isSafeInteger(internalPort) || internalPort < MIN_PORT || internalPort > MAX_PORT) {
    throw new Error(
      `ANTI_FRAUD_INTERNAL_PORT must be an integer in ${MIN_PORT}..${MAX_PORT}, got ${String(rawPort)}`,
    );
  }

  const rawHost = env.ANTI_FRAUD_INTERNAL_HOST?.trim();
  const internalHost = rawHost && rawHost.length > 0 ? rawHost : DEFAULT_INTERNAL_HOST;

  return {
    internalHost,
    internalPort,
    serviceToken,
    testHooksEnabled: env.FRAUD_TEST_HOOKS === 'true',
  };
}

export const RISK_CONFIG = Symbol('RISK_CONFIG');
