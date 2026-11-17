import { randomUUID } from 'node:crypto';
import { performance } from 'node:perf_hooks';
import type { INestApplication } from '@nestjs/common';
import {
  buildRulesFromConfig,
  type Clock,
  DecisionThresholds,
  ScoreRiskUseCase,
} from '@iris-gate/fraud';
import { PrismaFeatureSnapshotReader } from '../../infra/prisma/feature-snapshot.reader';
import type { PrismaService } from '../../infra/prisma/prisma.service';
import { PrismaRuleConfigStore } from '../../infra/prisma/rule-config.store';
import { PrismaScoredAssessmentStore } from '../../infra/prisma/scored-assessment.store';
import {
  connectTestDatabase,
  TEST_DATABASE_URL,
  uniqueCardFingerprint,
} from '../../infra/prisma/test-fixtures';
import {
  listenEphemeral,
  postRiskAssessment,
  TEST_SERVICE_TOKEN,
  testRiskConfig,
  validRiskRequestDto,
} from './risk.test-fixtures';
import type { RiskAssessmentResponseDto } from './risk-assessment.dto';
import { createInternalApp } from './risk.module';
import { ServiceTokenGuard } from './service-token.guard';

const WARMUP_REQUESTS = 10;
const BENCHMARK_REQUESTS = 100;
const P99_BUDGET_MS = 100;

const EXTERNAL_BENCHMARK_URL = process.env.RISK_BENCHMARK_URL;
const BENCHMARK_TOKEN = process.env.FRAUD_SERVICE_TOKEN ?? TEST_SERVICE_TOKEN;

const describeBenchmark = EXTERNAL_BENCHMARK_URL || TEST_DATABASE_URL ? describe : describe.skip;

function percentile(sortedMs: readonly number[], p: number): number {
  const index = Math.min(sortedMs.length - 1, Math.ceil((p / 100) * sortedMs.length) - 1);
  return sortedMs[index] ?? 0;
}

describeBenchmark('POST /internal/v1/risk-assessments p99 latency benchmark', () => {
  let prisma: PrismaService | null = null;
  let app: INestApplication | null = null;
  let baseUrl = EXTERNAL_BENCHMARK_URL ?? '';
  let authHeader = `Bearer ${BENCHMARK_TOKEN}`;
  const paymentIntentsUsed: string[] = [];

  beforeAll(async () => {
    if (EXTERNAL_BENCHMARK_URL) {
      return;
    }
    prisma = connectTestDatabase(TEST_DATABASE_URL ?? '');
    await prisma.$connect();
    const clock: Clock = { now: () => new Date() };
    const store = new PrismaScoredAssessmentStore(prisma);
    const reader = new PrismaFeatureSnapshotReader(prisma, clock);
    const ruleStore = new PrismaRuleConfigStore(prisma);
    const rules = buildRulesFromConfig(await ruleStore.load());
    const riskConfig = testRiskConfig();
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
    app = await createInternalApp({
      get: <T>(token: unknown): T => {
        if (token === ScoreRiskUseCase) return useCase as T;
        if (token === ServiceTokenGuard) return guard as T;
        return riskConfig as T;
      },
    } as never);
    baseUrl = await listenEphemeral(app);
    authHeader = `Bearer ${TEST_SERVICE_TOKEN}`;
  });

  afterAll(async () => {
    if (app) {
      await app.close();
    }
    if (prisma) {
      await prisma.paymentOutcome.deleteMany({
        where: { paymentIntentId: { in: paymentIntentsUsed } },
      });
      await prisma.riskAssessment.deleteMany({
        where: { paymentIntentId: { in: paymentIntentsUsed } },
      });
      await prisma.$disconnect();
    }
  });

  it(`keeps p99 latency below ${P99_BUDGET_MS} ms over ${BENCHMARK_REQUESTS} requests`, async () => {
    for (let i = 0; i < WARMUP_REQUESTS; i += 1) {
      const id = randomUUID();
      paymentIntentsUsed.push(id);
      const warmup = await postRiskAssessment<RiskAssessmentResponseDto>(
        baseUrl,
        validRiskRequestDto({
          payment_intent_id: id,
          card_fingerprint: uniqueCardFingerprint(),
        }),
        authHeader,
      );
      expect(warmup.status).toBe(200);
    }

    const durationsMs: number[] = [];
    for (let i = 0; i < BENCHMARK_REQUESTS; i += 1) {
      const id = randomUUID();
      paymentIntentsUsed.push(id);
      const start = performance.now();
      const res = await postRiskAssessment<RiskAssessmentResponseDto>(
        baseUrl,
        validRiskRequestDto({
          payment_intent_id: id,
          card_fingerprint: uniqueCardFingerprint(),
        }),
        authHeader,
      );
      const elapsedMs = performance.now() - start;
      expect(res.status).toBe(200);
      durationsMs.push(elapsedMs);
    }

    const sorted = [...durationsMs].sort((a, b) => a - b);
    const p50 = percentile(sorted, 50);
    const p95 = percentile(sorted, 95);
    const p99 = percentile(sorted, 99);
    const max = sorted[sorted.length - 1] ?? 0;

    process.stdout.write(
      `[risk-benchmark] n=${BENCHMARK_REQUESTS} p50=${p50.toFixed(2)}ms p95=${p95.toFixed(2)}ms p99=${p99.toFixed(2)}ms max=${max.toFixed(2)}ms\n`,
    );

    expect(p99).toBeLessThan(P99_BUDGET_MS);
  });
});
