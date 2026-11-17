import { Logger } from '@nestjs/common';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { RISK_CONFIG, type RiskConfig } from './config/risk.config';
import { createInternalApp } from './modules/risk/risk.module';

const DEFAULT_PUBLIC_PORT = 13001;

async function bootstrap(): Promise<void> {
  const app = await NestFactory.create(AppModule, { bufferLogs: true });
  const port = Number(process.env.PORT ?? DEFAULT_PUBLIC_PORT);
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

bootstrap().catch((err) => {
  console.error('bootstrap failed', err);
  process.exit(1);
});
