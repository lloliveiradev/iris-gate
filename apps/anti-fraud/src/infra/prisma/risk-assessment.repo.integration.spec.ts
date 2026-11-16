import { randomUUID } from 'node:crypto';
import { ASSESSMENT_COMPLETED_TOPIC, Decision, RiskAssessment } from '@iris-gate/fraud';
import type { PrismaService } from './prisma.service';
import { PrismaRiskAssessmentRepo } from './risk-assessment.repo';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  connectTestDatabase,
  TEST_CURRENCY,
  TEST_DATABASE_URL,
  TEST_MERCHANT_ID,
  TEST_NOW,
  uniqueCardFingerprint,
} from './test-fixtures';

const describeWithDatabase = TEST_DATABASE_URL ? describe : describe.skip;

describeWithDatabase('PrismaRiskAssessmentRepo (Postgres)', () => {
  let prisma: PrismaService;
  let repo: PrismaRiskAssessmentRepo;
  const assessmentId = randomUUID();
  const eventId = randomUUID();

  beforeAll(async () => {
    prisma = connectTestDatabase(TEST_DATABASE_URL ?? '');
    await prisma.$connect();
    repo = new PrismaRiskAssessmentRepo(prisma);
  });

  afterAll(async () => {
    await prisma.outboxMessage.deleteMany({ where: { eventId } });
    await prisma.riskAssessment.deleteMany({ where: { id: assessmentId } });
    await prisma.$disconnect();
  });

  it('stores an amount above 2^31 cents as BIGINT and reads it back exactly (AC3)', async () => {
    const assessment = RiskAssessment.create({
      id: assessmentId,
      paymentIntentId: randomUUID(),
      merchantId: TEST_MERCHANT_ID,
      accountRef: uniqueCardFingerprint(),
      amountCents: ABOVE_INT32_AMOUNT_CENTS,
      currency: TEST_CURRENCY,
      requestedAt: TEST_NOW,
    });
    assessment.complete(Decision.APPROVE, 0, [], TEST_NOW);

    await repo.saveCompletedWithOutbox(assessment, {
      eventId,
      eventType: ASSESSMENT_COMPLETED_TOPIC,
      topic: ASSESSMENT_COMPLETED_TOPIC,
      payload: {},
      occurredAt: TEST_NOW,
    });

    const row = await prisma.riskAssessment.findUniqueOrThrow({ where: { id: assessmentId } });
    const loaded = await repo.findById(assessmentId);

    expect(row.amountCents).toBe(BigInt(ABOVE_INT32_AMOUNT_CENTS));
    expect(loaded?.amountCents).toBe(ABOVE_INT32_AMOUNT_CENTS);
  });
});
