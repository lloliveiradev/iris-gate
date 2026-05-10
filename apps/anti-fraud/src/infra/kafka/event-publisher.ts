import { Injectable } from '@nestjs/common';
import type { EventPublisher, OutboxRecord } from '@iris-gate/fraud';
import { KafkaClient } from './kafka.client';

@Injectable()
export class KafkaEventPublisher implements EventPublisher {
  constructor(private readonly kafka: KafkaClient) {}

  async publish(record: OutboxRecord): Promise<void> {
    const headers: Record<string, string> = {
      'event-id': record.eventId,
      'event-type': record.eventType,
      'published-at': new Date().toISOString(),
    };
    if (record.traceparent) headers.traceparent = record.traceparent;

    await this.kafka.producer.send({
      topic: record.topic,
      messages: [
        {
          key: record.eventId,
          value: JSON.stringify(record.payload),
          headers,
        },
      ],
    });
  }
}
