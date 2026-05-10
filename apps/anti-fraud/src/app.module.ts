import { Module } from '@nestjs/common';
import { HealthController } from './health.controller';
import { KafkaModule } from './infra/kafka/kafka.module';
import { PrismaModule } from './infra/prisma/prisma.module';
import { FraudModule } from './modules/fraud/fraud.module';

@Module({
  imports: [PrismaModule, KafkaModule, FraudModule],
  controllers: [HealthController],
})
export class AppModule {}
