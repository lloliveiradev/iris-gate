import { Injectable } from '@nestjs/common';
import type { OutboxRecord, OutboxStore } from '@iris-gate/fraud';
import { PrismaService } from './prisma.service';

@Injectable()
export class PrismaOutboxStore implements OutboxStore {
  constructor(private readonly prisma: PrismaService) {}

  async pullUnpublished(limit: number): Promise<OutboxRecord[]> {
    const rows = await this.prisma.outboxMessage.findMany({
      where: { publishedAt: null },
      orderBy: { occurredAt: 'asc' },
      take: limit,
    });
    return rows.map((r) => ({
      eventId: r.eventId,
      eventType: r.eventType,
      topic: r.topic,
      payload: r.payload as unknown as Record<string, unknown>,
      occurredAt: r.occurredAt,
      traceparent: r.traceparent,
    }));
  }

  async markPublished(eventId: string, publishedAt: Date): Promise<void> {
    await this.prisma.outboxMessage.update({
      where: { eventId },
      data: { publishedAt },
    });
  }
}
