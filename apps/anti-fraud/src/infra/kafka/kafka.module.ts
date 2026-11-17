import { Global, Module } from '@nestjs/common';
import { KAFKA_CONFIG, loadKafkaConfig } from '../../config/kafka.config';
import { KafkaClient } from './kafka.client';
import { KafkaEventPublisher } from './event-publisher';

@Global()
@Module({
  providers: [
    { provide: KAFKA_CONFIG, useFactory: loadKafkaConfig },
    KafkaClient,
    KafkaEventPublisher,
  ],
  exports: [KAFKA_CONFIG, KafkaClient, KafkaEventPublisher],
})
export class KafkaModule {}
