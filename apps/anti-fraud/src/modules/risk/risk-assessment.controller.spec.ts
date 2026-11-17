import { randomUUID } from 'node:crypto';
import { Test } from '@nestjs/testing';
import { DEFAULT_RULES, FraudReasonCode, TestScenario } from '@iris-gate/fraud';
import { AppModule } from '../../app.module';
import { DATABASE_CONFIG } from '../../config/database.config';
import { RISK_CONFIG } from '../../config/risk.config';
import { KafkaClient } from '../../infra/kafka/kafka.client';
import { PrismaService } from '../../infra/prisma/prisma.service';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  BLOCKED_CARD_FINGERPRINT,
  CONFLICTING_CURRENCY,
  CONFLICTING_MERCHANT_ID,
  type InternalHttpHarness,
  listenEphemeral,
  postRiskAssessment,
  SCORING_LATENCY_STEP_MS,
  startInternalHttpHarness,
  TEST_CARD_FINGERPRINT,
  TEST_PAYMENT_INTENT_ID,
  testRiskConfig,
  validRiskRequestDto,
  WRONG_SAME_LENGTH_TOKEN,
  WRONG_SHORTER_TOKEN,
} from './risk.test-fixtures';
import type { ProblemDto, RiskAssessmentResponseDto } from './risk-assessment.dto';
import { RISK_ASSESSMENTS_PATH } from './risk-assessment.controller';
import { RiskModule } from './risk.module';

const DEFAULT_VELOCITY_LIMIT = 5;

describe('POST /internal/v1/risk-assessments (HTTP)', () => {
  let harness: InternalHttpHarness | null = null;

  afterEach(async () => {
    if (harness) {
      await harness.close();
      harness = null;
    }
  });

  describe('AC1: rules on a new payment intent', () => {
    it('approves a first-time card, reviews after N velocity attempts, and rejects a blocked card', async () => {
      harness = await startInternalHttpHarness();

      // 1. First-time card with a normal amount -> APPROVE (attempt 1 of 5 allowed).
      const first = await postRiskAssessment<RiskAssessmentResponseDto>(
        harness.baseUrl,
        validRiskRequestDto({ payment_intent_id: randomUUID() }),
      );
      expect(first.status).toBe(200);
      expect(first.body).toEqual({
        assessment_id: expect.stringMatching(
          /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
        ),
        decision: 'APPROVE',
        score: 0,
        reasons: [],
        latency_ms: SCORING_LATENCY_STEP_MS,
      });

      // Attempts 2..5 on the same card remain within maxAttempts24h (5).
      for (let attempt = 2; attempt <= DEFAULT_VELOCITY_LIMIT; attempt += 1) {
        const res = await postRiskAssessment<RiskAssessmentResponseDto>(
          harness.baseUrl,
          validRiskRequestDto({ payment_intent_id: randomUUID() }),
        );
        expect(res.status).toBe(200);
        expect(res.body.decision).toBe('APPROVE');
      }

      // 2. Attempt 6 on the same card exceeds maxAttempts24h (5) -> REVIEW with VELOCITY.
      const velocityReview = await postRiskAssessment<RiskAssessmentResponseDto>(
        harness.baseUrl,
        validRiskRequestDto({ payment_intent_id: randomUUID() }),
      );
      expect(velocityReview.status).toBe(200);
      expect(velocityReview.body.decision).toBe('REVIEW');
      expect(velocityReview.body.score).toBe(35);
      expect(velocityReview.body.reasons.map((r) => r.code)).toEqual(['VELOCITY']);

      // 3. A blocked card -> REJECT with BLOCKED_CARD.
      harness.features.blockCard(BLOCKED_CARD_FINGERPRINT);
      const blocked = await postRiskAssessment<RiskAssessmentResponseDto>(
        harness.baseUrl,
        validRiskRequestDto({
          payment_intent_id: randomUUID(),
          card_fingerprint: BLOCKED_CARD_FINGERPRINT,
        }),
      );
      expect(blocked.status).toBe(200);
      expect(blocked.body.decision).toBe('REJECT');
      expect(blocked.body.score).toBe(100);
      expect(blocked.body.reasons.map((r) => r.code)).toEqual(['BLOCKED_CARD']);
    });

    it('rejects a card over the velocity limit when VELOCITY weight is configured to reach reject', async () => {
      const rulesWithFullVelocityWeight = DEFAULT_RULES.map((rule) =>
        rule.code === FraudReasonCode.VELOCITY ? { ...rule, weight: 1.0 } : rule,
      );
      harness = await startInternalHttpHarness({ ruleConfigs: rulesWithFullVelocityWeight });

      for (let attempt = 1; attempt <= DEFAULT_VELOCITY_LIMIT; attempt += 1) {
        await postRiskAssessment(
          harness.baseUrl,
          validRiskRequestDto({ payment_intent_id: randomUUID() }),
        );
      }

      const velocityReject = await postRiskAssessment<RiskAssessmentResponseDto>(
        harness.baseUrl,
        validRiskRequestDto({ payment_intent_id: randomUUID() }),
      );
      expect(velocityReject.status).toBe(200);
      expect(velocityReject.body.decision).toBe('REJECT');
      expect(velocityReject.body.score).toBe(70);
      expect(velocityReject.body.reasons.map((r) => r.code)).toEqual(['VELOCITY']);
    });
  });

  describe('AC2: idempotency by payment_intent_id', () => {
    it('returns the stored assessment on a repeat and records no second attempted row', async () => {
      harness = await startInternalHttpHarness();

      const first = await postRiskAssessment<RiskAssessmentResponseDto>(
        harness.baseUrl,
        validRiskRequestDto(),
      );
      expect(first.status).toBe(200);
      expect(harness.store.saved).toHaveLength(1);

      // Even if the card is later blocked, a repeat for the same payment_intent_id
      // returns the stored assessment without rescoring or writing a second attempt.
      harness.features.blockCard(TEST_CARD_FINGERPRINT);
      const repeat = await postRiskAssessment<RiskAssessmentResponseDto>(
        harness.baseUrl,
        validRiskRequestDto(),
      );
      expect(repeat.status).toBe(200);
      expect(repeat.body).toEqual(first.body);
      expect(harness.store.saved).toHaveLength(1);
    });

    it.each([
      ['merchant_id', { merchant_id: CONFLICTING_MERCHANT_ID }],
      ['amount_cents', { amount_cents: ABOVE_INT32_AMOUNT_CENTS }],
      ['currency', { currency: CONFLICTING_CURRENCY }],
    ])(
      'returns 409 application/problem+json when a repeat conflicts on %s',
      async (_field, overrides) => {
        harness = await startInternalHttpHarness();
        await postRiskAssessment(harness.baseUrl, validRiskRequestDto());

        const conflict = await postRiskAssessment<ProblemDto>(
          harness.baseUrl,
          validRiskRequestDto(overrides),
        );
        expect(conflict.status).toBe(409);
        expect(conflict.contentType).toContain('application/problem+json');
        expect(conflict.body).toMatchObject({
          type: 'about:blank',
          title: 'assessment_conflict',
          status: 409,
        });
        expect(harness.store.saved).toHaveLength(1);
      },
    );
  });

  describe('AC3: 64-bit amounts above 2^31-1 cents', () => {
    it('scores and stores an amount above 21,474,836.47 accurately', async () => {
      harness = await startInternalHttpHarness();

      const res = await postRiskAssessment<RiskAssessmentResponseDto>(
        harness.baseUrl,
        validRiskRequestDto({ amount_cents: ABOVE_INT32_AMOUNT_CENTS }),
      );
      expect(res.status).toBe(200);
      expect(res.body.decision).toBe('APPROVE');
      expect(harness.store.saved[0]?.assessment.amountCents).toBe(ABOVE_INT32_AMOUNT_CENTS);
      expect(harness.store.saved[0]?.attempt.amountCents).toBe(ABOVE_INT32_AMOUNT_CENTS);
    });
  });

  describe('AC4: service token authentication and FRAUD_TEST_HOOKS gate', () => {
    it.each<[string, string | null]>([
      ['missing Authorization header', null],
      ['wrong Bearer token (same byte length)', `Bearer ${WRONG_SAME_LENGTH_TOKEN}`],
      ['wrong Bearer token (different byte length)', `Bearer ${WRONG_SHORTER_TOKEN}`],
    ])('returns 401 application/problem+json on %s', async (_label, authHeader) => {
      harness = await startInternalHttpHarness();

      const res = await postRiskAssessment<ProblemDto>(
        harness.baseUrl,
        validRiskRequestDto(),
        authHeader,
      );
      expect(res.status).toBe(401);
      expect(res.contentType).toContain('application/problem+json');
      expect(res.body).toEqual({
        type: 'about:blank',
        title: 'unauthorized',
        status: 401,
        detail: 'missing or invalid service token',
      });
      expect(harness.store.saved).toHaveLength(0);
    });

    it.each([TestScenario.BLOCKED, TestScenario.REVIEW])(
      'ignores test_scenario=%s when FRAUD_TEST_HOOKS=false',
      async (scenario) => {
        harness = await startInternalHttpHarness({ testHooksEnabled: false });

        const res = await postRiskAssessment<RiskAssessmentResponseDto>(
          harness.baseUrl,
          validRiskRequestDto({
            payment_intent_id: randomUUID(),
            test_scenario: scenario,
          }),
        );
        expect(res.status).toBe(200);
        expect(res.body.decision).toBe('APPROVE');
        expect(res.body.score).toBe(0);
        expect(res.body.reasons).toEqual([]);
      },
    );

    it.each<[TestScenario, 'REJECT' | 'REVIEW', number]>([
      [TestScenario.BLOCKED, 'REJECT', 70],
      [TestScenario.REVIEW, 'REVIEW', 30],
    ])(
      'honors test_scenario=%s with reason TEST_SCENARIO when FRAUD_TEST_HOOKS=true',
      async (scenario, expectedDecision, expectedScore) => {
        harness = await startInternalHttpHarness({ testHooksEnabled: true });

        const res = await postRiskAssessment<RiskAssessmentResponseDto>(
          harness.baseUrl,
          validRiskRequestDto({
            payment_intent_id: randomUUID(),
            test_scenario: scenario,
          }),
        );
        expect(res.status).toBe(200);
        expect(res.body.decision).toBe(expectedDecision);
        expect(res.body.score).toBe(expectedScore);
        expect(res.body.reasons).toEqual([
          expect.objectContaining({
            code: 'TEST_SCENARIO',
            weight: 1,
            score: expectedScore,
          }),
        ]);
      },
    );
  });

  describe('request validation and listener isolation (Q3)', () => {
    it('returns 400 application/problem+json on an invalid request body', async () => {
      harness = await startInternalHttpHarness();

      const res = await postRiskAssessment<ProblemDto>(harness.baseUrl, {
        ...validRiskRequestDto(),
        payment_intent_id: 'invalid-uuid',
      });
      expect(res.status).toBe(400);
      expect(res.contentType).toContain('application/problem+json');
      expect(res.body).toMatchObject({
        type: 'about:blank',
        title: 'validation',
        status: 400,
      });
    });

    it('keeps /internal/v1/risk-assessments off the public listener and serves it only on the internal listener', async () => {
      const publicModuleRef = await Test.createTestingModule({
        imports: [AppModule],
      })
        .overrideProvider(DATABASE_CONFIG)
        .useValue({ url: 'postgresql://localhost/antifraud' })
        .overrideProvider(PrismaService)
        .useValue({
          fraudRule: { findMany: async () => [] },
          outboxMessage: { findMany: async () => [] },
        })
        .overrideProvider(KafkaClient)
        .useValue({})
        .overrideProvider(RISK_CONFIG)
        .useValue(testRiskConfig())
        .compile();
      const publicApp = publicModuleRef.createNestApplication({ logger: false });
      const publicUrl = await listenEphemeral(publicApp);
      harness = await startInternalHttpHarness();

      try {
        const health = await fetch(`${publicUrl}/healthz`);
        expect(health.status).toBe(200);

        const publicRisk = await fetch(`${publicUrl}/${RISK_ASSESSMENTS_PATH}`, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify(validRiskRequestDto()),
        });
        expect(publicRisk.status).toBe(404);

        const internalRisk = await postRiskAssessment<RiskAssessmentResponseDto>(
          harness.baseUrl,
          validRiskRequestDto({ payment_intent_id: TEST_PAYMENT_INTENT_ID }),
        );
        expect(internalRisk.status).toBe(200);
        expect(internalRisk.body.decision).toBe('APPROVE');
      } finally {
        await publicApp.close();
      }
    });

    it('fails RiskModule boot when FRAUD_SERVICE_TOKEN is empty', async () => {
      const previousToken = process.env.FRAUD_SERVICE_TOKEN;
      delete process.env.FRAUD_SERVICE_TOKEN;
      try {
        await expect(
          Test.createTestingModule({
            imports: [RiskModule],
          })
            .overrideProvider(PrismaService)
            .useValue({
              fraudRule: { findMany: async () => [] },
            })
            .compile(),
        ).rejects.toThrow('FRAUD_SERVICE_TOKEN is not set');
      } finally {
        if (previousToken !== undefined) {
          process.env.FRAUD_SERVICE_TOKEN = previousToken;
        }
      }
    });
  });
});
