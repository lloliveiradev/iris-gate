import { Logger } from '@nestjs/common';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { parseTcpPort, RISK_CONFIG, type RiskConfig } from './config/risk.config';
import { createInternalApp } from './modules/risk/risk.module';

export const DEFAULT_PUBLIC_PORT = 13001;

/**
 * Reads `PORT` with the same 1..65535 integer check as `ANTI_FRAUD_INTERNAL_PORT`.
 * A missing or blank value uses `DEFAULT_PUBLIC_PORT`.
 */
export function readPublicPort(env: NodeJS.ProcessEnv = process.env): number {
  return parseTcpPort(env.PORT, 'PORT', DEFAULT_PUBLIC_PORT);
}

async function bootstrap(): Promise<void> {
  const app = await NestFactory.create(AppModule, { bufferLogs: true });
  const port = readPublicPort();
  const riskConfig = app.get<RiskConfig>(RISK_CONFIG);
  if (riskConfig.internalPort === port) {
    throw new Error(
      `ANTI_FRAUD_INTERNAL_PORT (${riskConfig.internalPort}) must differ from PORT (${port})`,
    );
  }

  const internalApp = await createInternalApp(app);

  app.enableShutdownHooks();
  internalApp.enableShutdownHooks();

  await app.listen(port);
  await internalApp.listen(riskConfig.internalPort, riskConfig.internalHost);

  Logger.log(`anti-fraud listening on :${port}`, 'Bootstrap');
  Logger.log(
    `anti-fraud internal API listening on ${riskConfig.internalHost}:${riskConfig.internalPort}`,
    'Bootstrap',
  );
}

// Unit tests import `readPublicPort` from this module. Start the listeners only
// when this file is the process entry point.
if (require.main === module) {
  bootstrap().catch((err) => {
    console.error('bootstrap failed', err);
    process.exit(1);
  });
}
