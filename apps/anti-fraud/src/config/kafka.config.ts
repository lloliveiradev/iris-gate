export interface KafkaConfig {
  clientId: string;
  brokers: string[];
  consumerGroup: string;
  topics: {
    completed: string;
    dlq: string;
  };
  outbox: {
    pollIntervalMs: number;
    batchSize: number;
  };
}

export function loadKafkaConfig(): KafkaConfig {
  const brokers = (process.env.KAFKA_BROKERS ?? 'localhost:19092')
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
  return {
    clientId: process.env.KAFKA_CLIENT_ID ?? 'iris-anti-fraud',
    brokers,
    consumerGroup: process.env.KAFKA_CONSUMER_GROUP ?? 'iris-anti-fraud',
    topics: {
      completed: process.env.KAFKA_TOPIC_COMPLETED ?? 'fraud.assessment.completed.v1',
      dlq: process.env.KAFKA_TOPIC_DLQ ?? 'fraud.assessment.failed.dlq',
    },
    outbox: {
      pollIntervalMs: Number(process.env.OUTBOX_POLL_INTERVAL_MS ?? 1000),
      batchSize: Number(process.env.OUTBOX_BATCH_SIZE ?? 50),
    },
  };
}

export const KAFKA_CONFIG = Symbol('KAFKA_CONFIG');
