import { Injectable } from '@nestjs/common';
import {
  type OutboxEventInput,
  type RiskAssessmentRepo,
  RiskAssessment,
  type FraudReason,
} from '@iris-gate/fraud';
import { PrismaService } from './prisma.service';

@Injectable()
export class PrismaRiskAssessmentRepo implements RiskAssessmentRepo {
  constructor(private readonly prisma: PrismaService) {}

  async findById(id: string): Promise<RiskAssessment | null> {
    const row = await this.prisma.riskAssessment.findUnique({ where: { id } });
    if (!row) return null;
    return RiskAssessment.rehydrate({
      id: row.id,
      paymentIntentId: row.paymentIntentId,
      merchantId: row.merchantId,
      accountRef: row.accountRef,
      amountCents: row.amountCents,
      currency: row.currency,
      status: row.status as 'PENDING' | 'COMPLETED' | 'FAILED',
      decision: row.decision as 'APPROVE' | 'REVIEW' | 'REJECT' | null,
      score: row.score,
      reasons: (row.reasons as unknown as FraudReason[]) ?? [],
      requestedAt: row.requestedAt,
      completedAt: row.completedAt,
    });
  }

  async saveCompletedWithOutbox(
    assessment: RiskAssessment,
    outboxEvent: OutboxEventInput,
  ): Promise<void> {
    const snap = assessment.toSnapshot();
    await this.prisma.$transaction([
      this.prisma.riskAssessment.upsert({
        where: { id: snap.id },
        create: {
          id: snap.id,
          paymentIntentId: snap.paymentIntentId,
          merchantId: snap.merchantId,
          accountRef: snap.accountRef,
          amountCents: snap.amountCents,
          currency: snap.currency,
          status: snap.status,
          decision: snap.decision,
          score: snap.score,
          reasons: snap.reasons as unknown as object,
          requestedAt: snap.requestedAt,
          completedAt: snap.completedAt,
        },
        update: {
          status: snap.status,
          decision: snap.decision,
          score: snap.score,
          reasons: snap.reasons as unknown as object,
          completedAt: snap.completedAt,
        },
      }),
      this.prisma.outboxMessage.create({
        data: {
          eventId: outboxEvent.eventId,
          eventType: outboxEvent.eventType,
          topic: outboxEvent.topic,
          payload: outboxEvent.payload as unknown as object,
          traceparent: outboxEvent.traceparent ?? null,
          occurredAt: outboxEvent.occurredAt,
        },
      }),
    ]);
  }

  async saveFailedWithOutbox(
    assessment: RiskAssessment,
    outboxEvent: OutboxEventInput,
  ): Promise<void> {
    return this.saveCompletedWithOutbox(assessment, outboxEvent);
  }
}
