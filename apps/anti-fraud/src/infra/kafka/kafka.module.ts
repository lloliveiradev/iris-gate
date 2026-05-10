import { Global, Module } from '@nestjs/common';
import { KAFKA_CONFIG, loadKafkaConfig } from '../../config/kafka.config';
import { KafkaClient } from './kafka.client';
import { KafkaDLQPublisher } from './dlq.publisher';
import { KafkaEventPublisher } from './event-publisher';

@Global()
@Module({
  providers: [
    { provide: KAFKA_CONFIG, useFactory: loadKafkaConfig },
    KafkaClient,
    KafkaEventPublisher,
    KafkaDLQPublisher,
  ],
  exports: [KAFKA_CONFIG, KafkaClient, KafkaEventPublisher, KafkaDLQPublisher],
})
export class KafkaModule {}
