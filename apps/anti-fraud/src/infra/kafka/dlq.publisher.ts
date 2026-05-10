import { Inject, Injectable } from '@nestjs/common';
import type { DLQPublisher } from '@iris-gate/fraud';
import { type KafkaConfig, KAFKA_CONFIG } from '../../config/kafka.config';
import { KafkaClient } from './kafka.client';

@Injectable()
export class KafkaDLQPublisher implements DLQPublisher {
  constructor(
    private readonly kafka: KafkaClient,
    @Inject(KAFKA_CONFIG) private readonly config: KafkaConfig,
  ) {}

  async publish(input: { topic: string; payload: unknown; reason: string }): Promise<void> {
    await this.kafka.producer.send({
      topic: input.topic || this.config.topics.dlq,
      messages: [
        {
          value: JSON.stringify({ payload: input.payload, reason: input.reason }),
          headers: {
            'failure-reason': input.reason,
            'failed-at': new Date().toISOString(),
          },
        },
      ],
    });
  }
}
