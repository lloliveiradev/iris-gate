import { Module } from '@nestjs/common';
import { HealthController } from './health.controller';
import { KafkaModule } from './infra/kafka/kafka.module';
import { PrismaModule } from './infra/prisma/prisma.module';
import { FraudModule } from './modules/fraud/fraud.module';
import { RiskModule } from './modules/risk/risk.module';

@Module({
  imports: [PrismaModule, KafkaModule, FraudModule, RiskModule],
  controllers: [HealthController],
})
export class AppModule {}
