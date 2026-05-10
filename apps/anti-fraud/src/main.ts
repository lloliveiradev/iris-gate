import { NestFactory } from '@nestjs/core';
import { Logger } from '@nestjs/common';
import { AppModule } from './app.module';

async function bootstrap(): Promise<void> {
  const app = await NestFactory.create(AppModule, { bufferLogs: true });
  const port = Number(process.env.PORT ?? 3001);

  app.enableShutdownHooks();
  await app.listen(port);

  Logger.log(`anti-fraud listening on :${port}`, 'Bootstrap');
}

bootstrap().catch((err) => {
  console.error('bootstrap failed', err);
  process.exit(1);
});
