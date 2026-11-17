import { randomUUID } from 'node:crypto';
import {
  ASSESSMENT_COMPLETED_TOPIC,
  createFeatureSnapshot,
  Decision,
  DuplicateAssessmentError,
  RiskAssessment,
  type ScoredAssessment,
} from '@iris-gate/fraud';
import { PaymentOutcomeKind } from './generated/client';
import type { PrismaService } from './prisma.service';
import { PrismaScoredAssessmentStore } from './scored-assessment.store';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  connectTestDatabase,
  TEST_CARD_BRAND,
  TEST_CURRENCY,
  TEST_DATABASE_URL,
  TEST_MERCHANT_ID,
  TEST_NOW,
  uniqueCardFingerprint,
} from './test-fixtures';

const describeWithDatabase = TEST_DATABASE_URL ? describe : describe.skip;

describeWithDatabase('PrismaScoredAssessmentStore (Postgres)', () => {
  let prisma: PrismaService;
  let store: PrismaScoredAssessmentStore;
  const paymentIntentsUsed: string[] = [];
  const eventsUsed: string[] = [];

  beforeAll(async () => {
    prisma = connectTestDatabase(TEST_DATABASE_URL ?? '');
    await prisma.$connect();
    store = new PrismaScoredAssessmentStore(prisma);
  });

  afterAll(async () => {
    await prisma.outboxMessage.deleteMany({ where: { eventId: { in: eventsUsed } } });
    await prisma.paymentOutcome.deleteMany({
      where: { paymentIntentId: { in: paymentIntentsUsed } },
    });
    await prisma.riskAssessment.deleteMany({
      where: { paymentIntentId: { in: paymentIntentsUsed } },
    });
    await prisma.$disconnect();
  });

  function buildScored(paymentIntentId: string, cardFingerprint: string): ScoredAssessment {
    const assessmentId = randomUUID();
    const eventId = randomUUID();
    const completedAt = new Date(TEST_NOW.getTime() + 15);
    paymentIntentsUsed.push(paymentIntentId);
    eventsUsed.push(eventId);

    const assessment = RiskAssessment.create({
      id: assessmentId,
      paymentIntentId,
      merchantId: TEST_MERCHANT_ID,
      accountRef: cardFingerprint,
      amountCents: ABOVE_INT32_AMOUNT_CENTS,
      currency: TEST_CURRENCY,
      requestedAt: TEST_NOW,
    });
    assessment.complete(Decision.APPROVE, 0, [], completedAt);

    return {
      assessment,
      features: createFeatureSnapshot({
        cardFingerprint,
        attempts24h: 0,
        declines24h: 0,
        avgCapturedCents: null,
        blocked: false,
      }),
      attempt: {
        paymentIntentId,
        merchantId: TEST_MERCHANT_ID,
        cardFingerprint,
        cardBrand: TEST_CARD_BRAND,
        customerRef: null,
        amountCents: ABOVE_INT32_AMOUNT_CENTS,
        currency: TEST_CURRENCY,
        occurredAt: TEST_NOW,
      },
      outboxEvent: {
        eventId,
        eventType: ASSESSMENT_COMPLETED_TOPIC,
        topic: ASSESSMENT_COMPLETED_TOPIC,
        occurredAt: completedAt,
        payload: {
          event_id: eventId,
          assessment_id: assessmentId,
          payment_intent_id: paymentIntentId,
        },
      },
    };
  }

  it('persists the assessment, features_snapshot, attempted outcome and outbox row in one transaction (AC2, AC3)', async () => {
    const paymentIntentId = randomUUID();
    const cardFingerprint = uniqueCardFingerprint();
    const first = buildScored(paymentIntentId, cardFingerprint);

    await store.saveScored(first);

    const loaded = await store.findByPaymentIntentId(paymentIntentId);
    expect(loaded?.id).toBe(first.assessment.id);
    expect(loaded?.amountCents).toBe(ABOVE_INT32_AMOUNT_CENTS);

    const rawAssessment = await prisma.riskAssessment.findUniqueOrThrow({
      where: { paymentIntentId },
    });
    expect(rawAssessment.featuresSnapshot).toEqual({
      cardFingerprint,
      attempts24h: 0,
      declines24h: 0,
      avgCapturedCents: null,
      blocked: false,
    });

    const duplicate = buildScored(paymentIntentId, cardFingerprint);
    await expect(store.saveScored(duplicate)).rejects.toThrow(DuplicateAssessmentError);

    const outcomeRows = await prisma.paymentOutcome.findMany({
      where: { paymentIntentId, outcome: PaymentOutcomeKind.attempted },
    });
    expect(outcomeRows).toHaveLength(1);

    const duplicateOutbox = await prisma.outboxMessage.findUnique({
      where: { eventId: duplicate.outboxEvent.eventId },
    });
    expect(duplicateOutbox).toBeNull();
  });
});
