import { Injectable } from '@nestjs/common';
import {
  DuplicateAssessmentError,
  type FeatureSnapshot,
  type FraudReason,
  RiskAssessment,
  type ScoredAssessment,
  type ScoredAssessmentStore,
} from '@iris-gate/fraud';
import { bigIntToSafeNumber, safeNumberToBigInt } from './bigint.mapping';
import { PaymentOutcomeKind, Prisma } from './generated/client';
import { PrismaService } from './prisma.service';

const UNIQUE_VIOLATION = 'P2002';

/**
 * Prisma adapter of `ScoredAssessmentStore`.
 *
 * Persists a completed `RiskAssessment` (with its `features_snapshot`), the
 * `attempted` row in `payment_outcomes`, and the `fraud.assessment.completed.v1`
 * outbox message in ONE `$transaction`. A unique-constraint collision on
 * `payment_intent_id` rolls back all three writes and throws
 * `DuplicateAssessmentError`.
 */
@Injectable()
export class PrismaScoredAssessmentStore implements ScoredAssessmentStore {
  constructor(private readonly prisma: PrismaService) {}

  async findByPaymentIntentId(paymentIntentId: string): Promise<RiskAssessment | null> {
    const row = await this.prisma.riskAssessment.findUnique({
      where: { paymentIntentId },
    });
    if (!row) return null;
    return RiskAssessment.rehydrate({
      id: row.id,
      paymentIntentId: row.paymentIntentId,
      merchantId: row.merchantId,
      accountRef: row.accountRef,
      amountCents: bigIntToSafeNumber('amountCents', row.amountCents),
      currency: row.currency,
      status: row.status as 'PENDING' | 'COMPLETED' | 'FAILED',
      decision: row.decision as 'APPROVE' | 'REVIEW' | 'REJECT' | null,
      score: row.score,
      reasons: (row.reasons as unknown as FraudReason[]) ?? [],
      requestedAt: row.requestedAt,
      completedAt: row.completedAt,
    });
  }

  async saveScored(scored: ScoredAssessment): Promise<void> {
    const snap = scored.assessment.toSnapshot();
    const assessmentAmount = safeNumberToBigInt('amountCents', snap.amountCents);
    const attemptAmount = safeNumberToBigInt('amountCents', scored.attempt.amountCents);

    try {
      await this.prisma.$transaction([
        this.prisma.riskAssessment.create({
          data: {
            id: snap.id,
            paymentIntentId: snap.paymentIntentId,
            merchantId: snap.merchantId,
            accountRef: snap.accountRef,
            amountCents: assessmentAmount,
            currency: snap.currency,
            status: snap.status,
            decision: snap.decision,
            score: snap.score,
            reasons: snap.reasons as unknown as Prisma.InputJsonValue,
            featuresSnapshot: toFeaturesSnapshotJson(scored.features),
            requestedAt: snap.requestedAt,
            completedAt: snap.completedAt,
          },
        }),
        this.prisma.paymentOutcome.create({
          data: {
            paymentIntentId: scored.attempt.paymentIntentId,
            outcome: PaymentOutcomeKind.attempted,
            merchantId: scored.attempt.merchantId,
            cardFingerprint: scored.attempt.cardFingerprint,
            cardBrand: scored.attempt.cardBrand,
            customerRef: scored.attempt.customerRef,
            amountCents: attemptAmount,
            currency: scored.attempt.currency,
            occurredAt: scored.attempt.occurredAt,
          },
        }),
        this.prisma.outboxMessage.create({
          data: {
            eventId: scored.outboxEvent.eventId,
            eventType: scored.outboxEvent.eventType,
            topic: scored.outboxEvent.topic,
            payload: scored.outboxEvent.payload as unknown as Prisma.InputJsonValue,
            traceparent: scored.outboxEvent.traceparent ?? null,
            occurredAt: scored.outboxEvent.occurredAt,
          },
        }),
      ]);
    } catch (err) {
      if (isUniqueViolation(err)) {
        throw new DuplicateAssessmentError(
          `assessment exists for payment intent ${snap.paymentIntentId}`,
        );
      }
      throw err;
    }
  }
}

function toFeaturesSnapshotJson(features: FeatureSnapshot): Prisma.InputJsonObject {
  return {
    cardFingerprint: features.cardFingerprint,
    attempts24h: features.attempts24h,
    declines24h: features.declines24h,
    avgCapturedCents: features.avgCapturedCents,
    blocked: features.blocked,
  };
}

function isUniqueViolation(err: unknown): boolean {
  if (err instanceof Prisma.PrismaClientKnownRequestError) {
    return err.code === UNIQUE_VIOLATION;
  }
  return (
    typeof err === 'object' &&
    err !== null &&
    'code' in err &&
    (err as { code?: unknown }).code === UNIQUE_VIOLATION
  );
}
