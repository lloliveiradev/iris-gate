import { Inject, Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { AssessRiskUseCase } from '@iris-gate/fraud';
import { type KafkaConfig, KAFKA_CONFIG } from '../../config/kafka.config';
import { KafkaDLQPublisher } from './dlq.publisher';
import { KafkaClient } from './kafka.client';
import {
  parseAssessmentRequest,
  PayloadValidationError,
  type RawAssessmentRequestPayload,
} from './payload';

@Injectable()
export class AssessmentConsumer implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(AssessmentConsumer.name);

  constructor(
    private readonly kafka: KafkaClient,
    @Inject(KAFKA_CONFIG) private readonly config: KafkaConfig,
    private readonly assessRisk: AssessRiskUseCase,
    private readonly dlq: KafkaDLQPublisher,
  ) {}

  async onModuleInit(): Promise<void> {
    await this.kafka.consumer.subscribe({
      topic: this.config.topics.request,
      fromBeginning: false,
    });

    await this.kafka.consumer.run({
      eachMessage: async ({ topic, partition, message }) => {
        const messageId = this.headerString(message.headers, 'event-id') ?? message.key?.toString();
        if (!messageId) {
          this.logger.warn(`message without event-id on ${topic}/${partition}, sending to DLQ`);
          await this.dlq.publish({
            topic: this.config.topics.dlq,
            payload: message.value?.toString() ?? null,
            reason: 'missing event-id header',
          });
          return;
        }

        let raw: RawAssessmentRequestPayload;
        try {
          raw = JSON.parse(message.value?.toString() ?? '{}') as RawAssessmentRequestPayload;
        } catch (err) {
          this.logger.warn(`malformed JSON on ${topic}/${partition}: ${(err as Error).message}`);
          await this.dlq.publish({
            topic: this.config.topics.dlq,
            payload: message.value?.toString() ?? null,
            reason: `malformed JSON: ${(err as Error).message}`,
          });
          return;
        }

        let parsed: ReturnType<typeof parseAssessmentRequest>;
        try {
          parsed = parseAssessmentRequest(raw);
        } catch (err) {
          if (err instanceof PayloadValidationError) {
            this.logger.warn(`invalid payload (${messageId}): ${err.message}`);
            await this.dlq.publish({
              topic: this.config.topics.dlq,
              payload: raw,
              reason: err.message,
            });
            return;
          }
          throw err;
        }

        const traceparent = this.headerString(message.headers, 'traceparent');
        const result = await this.assessRisk.execute({
          messageId,
          assessmentId: parsed.assessmentId,
          paymentIntentId: parsed.paymentIntentId,
          merchantId: parsed.merchantId,
          accountRef: parsed.accountRef,
          amountCents: parsed.amountCents,
          currency: parsed.currency,
          occurredAt: parsed.occurredAt,
          traceparent,
        });
        this.logger.log(
          `assessed ${parsed.assessmentId} → ${result.decision} (score=${result.score.toFixed(1)}, dedup=${result.deduplicated})`,
        );
      },
    });

    this.logger.log(`consuming ${this.config.topics.request}`);
  }

  async onModuleDestroy(): Promise<void> {
    await this.kafka.consumer.stop();
  }

  private headerString(
    headers: Record<string, Buffer | string | (Buffer | string)[] | undefined> | undefined,
    key: string,
  ): string | undefined {
    const v = headers?.[key];
    if (v === undefined) return undefined;
    if (Array.isArray(v)) return v[0]?.toString();
    return typeof v === 'string' ? v : v.toString();
  }
}
