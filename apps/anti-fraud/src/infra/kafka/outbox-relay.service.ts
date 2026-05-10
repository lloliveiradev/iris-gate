import { Inject, Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import type { OutboxStore } from '@iris-gate/fraud';
import { type KafkaConfig, KAFKA_CONFIG } from '../../config/kafka.config';
import { KafkaEventPublisher } from './event-publisher';

export const OUTBOX_STORE = Symbol('OUTBOX_STORE');

@Injectable()
export class OutboxRelayService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(OutboxRelayService.name);
  private timer: NodeJS.Timeout | null = null;
  private running = false;

  constructor(
    @Inject(OUTBOX_STORE) private readonly store: OutboxStore,
    private readonly publisher: KafkaEventPublisher,
    @Inject(KAFKA_CONFIG) private readonly config: KafkaConfig,
  ) {}

  onModuleInit(): void {
    this.running = true;
    this.scheduleNextTick();
    this.logger.log(
      `outbox relay started (interval=${this.config.outbox.pollIntervalMs}ms, batch=${this.config.outbox.batchSize})`,
    );
  }

  async onModuleDestroy(): Promise<void> {
    this.running = false;
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
    // best-effort drain on shutdown
    await this.tick().catch((err) =>
      this.logger.warn(`final tick failed: ${(err as Error).message}`),
    );
  }

  private scheduleNextTick(): void {
    if (!this.running) return;
    this.timer = setTimeout(() => {
      this.tick()
        .catch((err) => this.logger.warn(`relay tick failed: ${(err as Error).message}`))
        .finally(() => this.scheduleNextTick());
    }, this.config.outbox.pollIntervalMs);
  }

  async tick(): Promise<number> {
    const records = await this.store.pullUnpublished(this.config.outbox.batchSize);
    if (records.length === 0) return 0;
    let published = 0;
    for (const rec of records) {
      try {
        await this.publisher.publish(rec);
        await this.store.markPublished(rec.eventId, new Date());
        published += 1;
      } catch (err) {
        this.logger.warn(
          `publish failed for ${rec.eventId}: ${(err as Error).message} (will retry)`,
        );
        break;
      }
    }
    return published;
  }
}
