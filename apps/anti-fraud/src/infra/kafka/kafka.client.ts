import { Inject, Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { Consumer, Kafka, Producer } from 'kafkajs';
import { type KafkaConfig, KAFKA_CONFIG } from '../../config/kafka.config';

@Injectable()
export class KafkaClient implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(KafkaClient.name);
  private readonly kafka: Kafka;
  private readonly _producer: Producer;
  private readonly _consumer: Consumer;
  private connected = false;

  constructor(@Inject(KAFKA_CONFIG) private readonly config: KafkaConfig) {
    this.kafka = new Kafka({ clientId: config.clientId, brokers: config.brokers });
    this._producer = this.kafka.producer({ allowAutoTopicCreation: false });
    this._consumer = this.kafka.consumer({ groupId: config.consumerGroup });
  }

  get producer(): Producer {
    return this._producer;
  }

  get consumer(): Consumer {
    return this._consumer;
  }

  async onModuleInit(): Promise<void> {
    await this._producer.connect();
    await this._consumer.connect();
    this.connected = true;
    this.logger.log(`kafka connected (brokers=${this.config.brokers.join(',')})`);
  }

  async onModuleDestroy(): Promise<void> {
    if (!this.connected) return;
    await this._consumer.disconnect();
    await this._producer.disconnect();
    this.logger.log('kafka disconnected');
  }
}
