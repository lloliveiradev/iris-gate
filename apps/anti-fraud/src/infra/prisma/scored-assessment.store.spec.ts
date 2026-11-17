import {
  ASSESSMENT_COMPLETED_TOPIC,
  createFeatureSnapshot,
  Decision,
  DuplicateAssessmentError,
  FraudReasonCode,
  RiskAssessment,
  type ScoredAssessment,
} from '@iris-gate/fraud';
import { UnsafeIntegerError } from './bigint.mapping';
import { PaymentOutcomeKind } from './generated/client';
import type { PrismaService } from './prisma.service';
import { PrismaScoredAssessmentStore } from './scored-assessment.store';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  ABOVE_SAFE_INTEGER,
  TEST_CARD_BRAND,
  TEST_CURRENCY,
  TEST_MERCHANT_ID,
  TEST_NOW,
} from './test-fixtures';
import {
  TEST_ASSESSMENT_ID,
  TEST_CARD_FINGERPRINT,
  TEST_CUSTOMER_REF,
  TEST_EVENT_ID,
  TEST_PAYMENT_INTENT_ID,
} from '../../modules/risk/risk.test-fixtures';

const COMPLETED_AT = new Date(TEST_NOW.getTime() + 10);

function sampleScoredAssessment(): ScoredAssessment {
  const assessment = RiskAssessment.create({
    id: TEST_ASSESSMENT_ID,
    paymentIntentId: TEST_PAYMENT_INTENT_ID,
    merchantId: TEST_MERCHANT_ID,
    accountRef: TEST_CARD_FINGERPRINT,
    amountCents: ABOVE_INT32_AMOUNT_CENTS,
    currency: TEST_CURRENCY,
    requestedAt: TEST_NOW,
  });
  assessment.complete(
    Decision.REJECT,
    100,
    [
      {
        code: FraudReasonCode.BLOCKED_CARD,
        weight: 1,
        score: 100,
        description: 'card is blocked',
      },
    ],
    COMPLETED_AT,
  );
  const features = createFeatureSnapshot({
    cardFingerprint: TEST_CARD_FINGERPRINT,
    attempts24h: 2,
    declines24h: 1,
    avgCapturedCents: 1_500,
    blocked: true,
  });
  return {
    assessment,
    features,
    attempt: {
      paymentIntentId: TEST_PAYMENT_INTENT_ID,
      merchantId: TEST_MERCHANT_ID,
      cardFingerprint: TEST_CARD_FINGERPRINT,
      cardBrand: TEST_CARD_BRAND,
      customerRef: TEST_CUSTOMER_REF,
      amountCents: ABOVE_INT32_AMOUNT_CENTS,
      currency: TEST_CURRENCY,
      occurredAt: TEST_NOW,
    },
    outboxEvent: {
      eventId: TEST_EVENT_ID,
      eventType: ASSESSMENT_COMPLETED_TOPIC,
      topic: ASSESSMENT_COMPLETED_TOPIC,
      occurredAt: COMPLETED_AT,
      payload: {
        event_id: TEST_EVENT_ID,
        assessment_id: TEST_ASSESSMENT_ID,
      },
    },
  };
}

describe('PrismaScoredAssessmentStore (unit)', () => {
  it('returns null when no assessment is stored for the payment intent', async () => {
    const findUnique = jest.fn().mockResolvedValue(null);
    const prisma = { riskAssessment: { findUnique } } as unknown as PrismaService;
    const store = new PrismaScoredAssessmentStore(prisma);

    await expect(store.findByPaymentIntentId(TEST_PAYMENT_INTENT_ID)).resolves.toBeNull();
    expect(findUnique).toHaveBeenCalledWith({
      where: { paymentIntentId: TEST_PAYMENT_INTENT_ID },
    });
  });

  it('rehydrates a stored assessment with BIGINT amountCents above 2^31', async () => {
    const findUnique = jest.fn().mockResolvedValue({
      id: TEST_ASSESSMENT_ID,
      paymentIntentId: TEST_PAYMENT_INTENT_ID,
      merchantId: TEST_MERCHANT_ID,
      accountRef: TEST_CARD_FINGERPRINT,
      amountCents: BigInt(ABOVE_INT32_AMOUNT_CENTS),
      currency: TEST_CURRENCY,
      status: 'COMPLETED',
      decision: 'APPROVE',
      score: 0,
      reasons: [],
      requestedAt: TEST_NOW,
      completedAt: COMPLETED_AT,
    });
    const prisma = { riskAssessment: { findUnique } } as unknown as PrismaService;
    const store = new PrismaScoredAssessmentStore(prisma);

    const assessment = await store.findByPaymentIntentId(TEST_PAYMENT_INTENT_ID);
    expect(assessment?.toSnapshot()).toEqual({
      id: TEST_ASSESSMENT_ID,
      paymentIntentId: TEST_PAYMENT_INTENT_ID,
      merchantId: TEST_MERCHANT_ID,
      accountRef: TEST_CARD_FINGERPRINT,
      amountCents: ABOVE_INT32_AMOUNT_CENTS,
      currency: TEST_CURRENCY,
      status: 'COMPLETED',
      decision: 'APPROVE',
      score: 0,
      reasons: [],
      requestedAt: TEST_NOW,
      completedAt: COMPLETED_AT,
    });
  });

  it('fails closed when the stored BIGINT amount exceeds MAX_SAFE_INTEGER', async () => {
    const findUnique = jest.fn().mockResolvedValue({
      id: TEST_ASSESSMENT_ID,
      paymentIntentId: TEST_PAYMENT_INTENT_ID,
      merchantId: TEST_MERCHANT_ID,
      accountRef: TEST_CARD_FINGERPRINT,
      amountCents: ABOVE_SAFE_INTEGER,
      currency: TEST_CURRENCY,
      status: 'COMPLETED',
      decision: 'APPROVE',
      score: 0,
      reasons: [],
      requestedAt: TEST_NOW,
      completedAt: COMPLETED_AT,
    });
    const prisma = { riskAssessment: { findUnique } } as unknown as PrismaService;
    const store = new PrismaScoredAssessmentStore(prisma);

    await expect(store.findByPaymentIntentId(TEST_PAYMENT_INTENT_ID)).rejects.toThrow(
      UnsafeIntegerError,
    );
  });

  it('writes the assessment, snapshot, attempted outcome, and outbox event in one $transaction', async () => {
    const createAssessment = jest.fn().mockReturnValue('op_assessment');
    const createOutcome = jest.fn().mockReturnValue('op_outcome');
    const createOutbox = jest.fn().mockReturnValue('op_outbox');
    const transaction = jest.fn().mockResolvedValue(undefined);
    const prisma = {
      riskAssessment: { create: createAssessment },
      paymentOutcome: { create: createOutcome },
      outboxMessage: { create: createOutbox },
      $transaction: transaction,
    } as unknown as PrismaService;
    const store = new PrismaScoredAssessmentStore(prisma);

    await store.saveScored(sampleScoredAssessment());

    expect(createAssessment).toHaveBeenCalledWith({
      data: expect.objectContaining({
        id: TEST_ASSESSMENT_ID,
        paymentIntentId: TEST_PAYMENT_INTENT_ID,
        amountCents: BigInt(ABOVE_INT32_AMOUNT_CENTS),
        decision: 'REJECT',
        featuresSnapshot: {
          cardFingerprint: TEST_CARD_FINGERPRINT,
          attempts24h: 2,
          declines24h: 1,
          avgCapturedCents: 1_500,
          blocked: true,
        },
      }),
    });
    expect(createOutcome).toHaveBeenCalledWith({
      data: {
        paymentIntentId: TEST_PAYMENT_INTENT_ID,
        outcome: PaymentOutcomeKind.attempted,
        merchantId: TEST_MERCHANT_ID,
        cardFingerprint: TEST_CARD_FINGERPRINT,
        cardBrand: TEST_CARD_BRAND,
        customerRef: TEST_CUSTOMER_REF,
        amountCents: BigInt(ABOVE_INT32_AMOUNT_CENTS),
        currency: TEST_CURRENCY,
        occurredAt: TEST_NOW,
      },
    });
    expect(createOutbox).toHaveBeenCalledWith({
      data: {
        eventId: TEST_EVENT_ID,
        eventType: ASSESSMENT_COMPLETED_TOPIC,
        topic: ASSESSMENT_COMPLETED_TOPIC,
        payload: {
          event_id: TEST_EVENT_ID,
          assessment_id: TEST_ASSESSMENT_ID,
        },
        traceparent: null,
        occurredAt: COMPLETED_AT,
      },
    });
    expect(transaction).toHaveBeenCalledTimes(1);
    expect(transaction).toHaveBeenCalledWith(['op_assessment', 'op_outcome', 'op_outbox']);
  });

  it('translates a P2002 unique-constraint error into DuplicateAssessmentError', async () => {
    const prisma = {
      riskAssessment: { create: jest.fn() },
      paymentOutcome: { create: jest.fn() },
      outboxMessage: { create: jest.fn() },
      $transaction: jest.fn().mockRejectedValue({ code: 'P2002' }),
    } as unknown as PrismaService;
    const store = new PrismaScoredAssessmentStore(prisma);

    await expect(store.saveScored(sampleScoredAssessment())).rejects.toThrow(
      DuplicateAssessmentError,
    );
  });

  it('rethrows non-unique database errors unchanged', async () => {
    const dbError = new Error('connection lost');
    const prisma = {
      riskAssessment: { create: jest.fn() },
      paymentOutcome: { create: jest.fn() },
      outboxMessage: { create: jest.fn() },
      $transaction: jest.fn().mockRejectedValue(dbError),
    } as unknown as PrismaService;
    const store = new PrismaScoredAssessmentStore(prisma);

    await expect(store.saveScored(sampleScoredAssessment())).rejects.toThrow(dbError);
  });
});
