import { randomUUID } from 'node:crypto';
import type { INestApplication } from '@nestjs/common';
import {
  buildRulesFromConfig,
  type Clock,
  DecisionThresholds,
  ScoreRiskUseCase,
  TestScenario,
} from '@iris-gate/fraud';
import { PrismaFeatureSnapshotReader } from '../../infra/prisma/feature-snapshot.reader';
import { PaymentOutcomeKind } from '../../infra/prisma/generated/client';
import type { PrismaService } from '../../infra/prisma/prisma.service';
import { PrismaRuleConfigStore } from '../../infra/prisma/rule-config.store';
import { PrismaScoredAssessmentStore } from '../../infra/prisma/scored-assessment.store';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  connectTestDatabase,
  TEST_DATABASE_URL,
  uniqueCardFingerprint,
} from '../../infra/prisma/test-fixtures';
import {
  listenEphemeral,
  postRiskAssessment,
  testRiskConfig,
  validRiskRequestDto,
} from './risk.test-fixtures';
import type { ProblemDto, RiskAssessmentResponseDto } from './risk-assessment.dto';
import { createInternalApp } from './risk.module';
import { ServiceTokenGuard } from './service-token.guard';

const DEFAULT_VELOCITY_LIMIT = 5;
const describeWithDatabase = TEST_DATABASE_URL ? describe : describe.skip;

describeWithDatabase('POST /internal/v1/risk-assessments (Postgres integration)', () => {
  let prisma: PrismaService;
  const paymentIntentsUsed: string[] = [];
  const cardsUsed: string[] = [];

  beforeAll(async () => {
    prisma = connectTestDatabase(TEST_DATABASE_URL ?? '');
    await prisma.$connect();
  });

  afterAll(async () => {
    await prisma.outboxMessage.deleteMany({
      where: {
        createdAt: { gte: new Date(Date.now() - 60_000) },
      },
    });
    await prisma.paymentOutcome.deleteMany({
      where: { paymentIntentId: { in: paymentIntentsUsed } },
    });
    await prisma.riskAssessment.deleteMany({
      where: { paymentIntentId: { in: paymentIntentsUsed } },
    });
    await prisma.cardProfile.deleteMany({
      where: { cardFingerprint: { in: cardsUsed } },
    });
    await prisma.$disconnect();
  });

  function newCard(): string {
    const card = uniqueCardFingerprint();
    cardsUsed.push(card);
    return card;
  }

  function newIntent(): string {
    const id = randomUUID();
    paymentIntentsUsed.push(id);
    return id;
  }

  async function startPostgresInternalApp(testHooksEnabled = false): Promise<{
    app: INestApplication;
    baseUrl: string;
  }> {
    const clock: Clock = { now: () => new Date() };
    const store = new PrismaScoredAssessmentStore(prisma);
    const reader = new PrismaFeatureSnapshotReader(prisma, clock);
    const ruleStore = new PrismaRuleConfigStore(prisma);
    const rules = buildRulesFromConfig(await ruleStore.load());
    const riskConfig = testRiskConfig({ testHooksEnabled });
    const useCase = new ScoreRiskUseCase(
      store,
      reader,
      rules,
      DecisionThresholds.defaults(),
      clock,
      { newId: () => randomUUID() },
      riskConfig.testHooksEnabled,
    );
    const guard = new ServiceTokenGuard(riskConfig);
    const app = await createInternalApp({
      get: <T>(token: unknown): T => {
        if (token === ScoreRiskUseCase) return useCase as T;
        if (token === ServiceTokenGuard) return guard as T;
        return riskConfig as T;
      },
    } as never);
    const baseUrl = await listenEphemeral(app);
    return { app, baseUrl };
  }

  it('satisfies AC1, AC2, AC3, and AC4 against Postgres', async () => {
    const { app, baseUrl } = await startPostgresInternalApp(false);
    try {
      // AC4: missing service token -> 401.
      const unauth = await postRiskAssessment<ProblemDto>(
        baseUrl,
        validRiskRequestDto({ payment_intent_id: newIntent() }),
        null,
      );
      expect(unauth.status).toBe(401);

      // AC1 + AC2: first-time card gets APPROVE, repeat returns same assessment_id and 1 attempted row.
      const card = newCard();
      const firstIntentId = newIntent();
      const first = await postRiskAssessment<RiskAssessmentResponseDto>(
        baseUrl,
        validRiskRequestDto({
          payment_intent_id: firstIntentId,
          card_fingerprint: card,
          test_scenario: TestScenario.BLOCKED, // ignored because testHooksEnabled = false (AC4)
        }),
      );
      expect(first.status).toBe(200);
      expect(first.body.decision).toBe('APPROVE');

      const repeat = await postRiskAssessment<RiskAssessmentResponseDto>(
        baseUrl,
        validRiskRequestDto({
          payment_intent_id: firstIntentId,
          card_fingerprint: card,
        }),
      );
      expect(repeat.status).toBe(200);
      expect(repeat.body.assessment_id).toBe(first.body.assessment_id);

      const attemptedCount = await prisma.paymentOutcome.count({
        where: { paymentIntentId: firstIntentId, outcome: PaymentOutcomeKind.attempted },
      });
      expect(attemptedCount).toBe(1);

      // AC1: 4 more attempts (total 5) still APPROVE; 6th attempt gets REVIEW (VELOCITY).
      for (let i = 2; i <= DEFAULT_VELOCITY_LIMIT; i += 1) {
        const res = await postRiskAssessment<RiskAssessmentResponseDto>(
          baseUrl,
          validRiskRequestDto({
            payment_intent_id: newIntent(),
            card_fingerprint: card,
          }),
        );
        expect(res.body.decision).toBe('APPROVE');
      }

      const sixth = await postRiskAssessment<RiskAssessmentResponseDto>(
        baseUrl,
        validRiskRequestDto({
          payment_intent_id: newIntent(),
          card_fingerprint: card,
        }),
      );
      expect(sixth.status).toBe(200);
      expect(sixth.body.decision).toBe('REVIEW');
      expect(sixth.body.reasons.map((r) => r.code)).toEqual(['VELOCITY']);

      // AC1: blocked card in card_profiles -> REJECT.
      const blockedCard = newCard();
      await prisma.cardProfile.create({
        data: { cardFingerprint: blockedCard, blocked: true },
      });
      const blockedRes = await postRiskAssessment<RiskAssessmentResponseDto>(
        baseUrl,
        validRiskRequestDto({
          payment_intent_id: newIntent(),
          card_fingerprint: blockedCard,
        }),
      );
      expect(blockedRes.status).toBe(200);
      expect(blockedRes.body.decision).toBe('REJECT');
      expect(blockedRes.body.reasons.map((r) => r.code)).toEqual(['BLOCKED_CARD']);

      // AC3: amount above 2^31 cents -> stored and scored accurately.
      const largeIntentId = newIntent();
      const largeRes = await postRiskAssessment<RiskAssessmentResponseDto>(
        baseUrl,
        validRiskRequestDto({
          payment_intent_id: largeIntentId,
          card_fingerprint: newCard(),
          amount_cents: ABOVE_INT32_AMOUNT_CENTS,
        }),
      );
      expect(largeRes.status).toBe(200);
      expect(largeRes.body.decision).toBe('APPROVE');
      const storedLarge = await prisma.riskAssessment.findUniqueOrThrow({
        where: { paymentIntentId: largeIntentId },
      });
      expect(storedLarge.amountCents).toBe(BigInt(ABOVE_INT32_AMOUNT_CENTS));
    } finally {
      await app.close();
    }
  });
});
