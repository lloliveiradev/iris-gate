import { randomUUID } from 'node:crypto';
import type { AddressInfo } from 'node:net';
import type { INestApplication } from '@nestjs/common';
import {
  buildRulesFromConfig,
  type Clock,
  createFeatureSnapshot,
  DecisionThresholds,
  DEFAULT_RULES,
  DuplicateAssessmentError,
  type FeatureSnapshot,
  type FeatureSnapshotQuery,
  type FeatureSnapshotReader,
  RiskAssessment,
  type RiskAssessmentSnapshot,
  type RuleConfig,
  type ScoredAssessment,
  type ScoredAssessmentStore,
  ScoreRiskUseCase,
} from '@iris-gate/fraud';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  TEST_AMOUNT_CENTS,
  TEST_CARD_BRAND,
  TEST_CURRENCY,
  TEST_MERCHANT_ID,
  TEST_NOW,
} from '../../infra/prisma/test-fixtures';
import {
  DEFAULT_INTERNAL_HOST,
  DEFAULT_INTERNAL_PORT,
  type RiskConfig,
} from '../../config/risk.config';
import { RISK_ASSESSMENTS_PATH } from './risk-assessment.controller';
import type {
  ProblemDto,
  RiskAssessmentRequestDto,
  RiskAssessmentResponseDto,
} from './risk-assessment.dto';
import { createInternalApp } from './risk.module';
import { ServiceTokenGuard } from './service-token.guard';

export {
  ABOVE_INT32_AMOUNT_CENTS,
  TEST_AMOUNT_CENTS,
  TEST_CARD_BRAND,
  TEST_CURRENCY,
  TEST_MERCHANT_ID,
  TEST_NOW,
};

/** Shared fake service tokens for the internal risk-scoring specs. */
export const TEST_SERVICE_TOKEN = 'svc_test_fraud_token'; // gitleaks:allow
export const WRONG_SAME_LENGTH_TOKEN = 'svc_test_wrong_token'; // gitleaks:allow
export const WRONG_SHORTER_TOKEN = 'svc_short'; // gitleaks:allow
export const WRONG_LONGER_TOKEN = 'svc_test_fraud_token_extra'; // gitleaks:allow
export const MULTIBYTE_WRONG_TOKEN = 'svc_test_fraud_toké'; // gitleaks:allow

export const TEST_PAYMENT_INTENT_ID = '00000000-0000-4000-8000-0000000000e1';
export const OTHER_PAYMENT_INTENT_ID = '00000000-0000-4000-8000-0000000000e2';
export const CONFLICTING_MERCHANT_ID = '00000000-0000-4000-8000-0000000000d2';
export const CONFLICTING_CURRENCY = 'USD';
export const TEST_CARD_FINGERPRINT = 'fp_test_card_001';
export const BLOCKED_CARD_FINGERPRINT = 'fp_test_blocked_001';
export const TEST_CUSTOMER_REF = 'cus_test_001';
export const TEST_ASSESSMENT_ID = '00000000-0000-4000-8000-0000000000f1';
export const TEST_EVENT_ID = '00000000-0000-4000-8000-0000000000f2';
export const SCORING_LATENCY_STEP_MS = 5;

export function testRiskConfig(overrides: Partial<RiskConfig> = {}): RiskConfig {
  return {
    internalHost: DEFAULT_INTERNAL_HOST,
    internalPort: DEFAULT_INTERNAL_PORT,
    serviceToken: TEST_SERVICE_TOKEN,
    testHooksEnabled: false,
    ...overrides,
  };
}

export function validRiskRequestDto(
  overrides: Partial<RiskAssessmentRequestDto> = {},
): RiskAssessmentRequestDto {
  return {
    payment_intent_id: TEST_PAYMENT_INTENT_ID,
    merchant_id: TEST_MERCHANT_ID,
    amount_cents: TEST_AMOUNT_CENTS,
    currency: TEST_CURRENCY,
    card_fingerprint: TEST_CARD_FINGERPRINT,
    card_brand: TEST_CARD_BRAND,
    customer_ref: TEST_CUSTOMER_REF,
    ...overrides,
  };
}

/**
 * In-memory `ScoredAssessmentStore` for controller tests. Records every
 * `saveScored` call (including the `attempted` outcome and outbox event) and
 * rejects duplicate `payment_intent_id`s with `DuplicateAssessmentError`.
 */
export class InMemoryTestAssessmentStore implements ScoredAssessmentStore {
  readonly saved: ScoredAssessment[] = [];
  private readonly byPaymentIntent = new Map<string, RiskAssessmentSnapshot>();

  async findByPaymentIntentId(paymentIntentId: string): Promise<RiskAssessment | null> {
    const snap = this.byPaymentIntent.get(paymentIntentId);
    return snap ? RiskAssessment.rehydrate(snap) : null;
  }

  async saveScored(scored: ScoredAssessment): Promise<void> {
    const { paymentIntentId } = scored.assessment;
    if (this.byPaymentIntent.has(paymentIntentId)) {
      throw new DuplicateAssessmentError(`assessment exists for payment intent ${paymentIntentId}`);
    }
    this.byPaymentIntent.set(paymentIntentId, scored.assessment.toSnapshot());
    this.saved.push(scored);
  }
}

/**
 * In-memory `FeatureSnapshotReader` backed by the `attempted` outcomes stored in
 * `InMemoryTestAssessmentStore`, plus card-level overrides (`blocked`, `declines24h`,
 * `avgCapturedCents`).
 */
export class InMemoryTestFeatureReader implements FeatureSnapshotReader {
  private readonly blockedCards = new Set<string>();
  private readonly declinesByCard = new Map<string, number>();
  private readonly avgCapturedByCard = new Map<string, number | null>();

  constructor(private readonly store: InMemoryTestAssessmentStore) {}

  blockCard(cardFingerprint: string): void {
    this.blockedCards.add(cardFingerprint);
  }

  setDeclines24h(cardFingerprint: string, count: number): void {
    this.declinesByCard.set(cardFingerprint, count);
  }

  setAvgCapturedCents(cardFingerprint: string, avg: number | null): void {
    this.avgCapturedByCard.set(cardFingerprint, avg);
  }

  async load(query: FeatureSnapshotQuery): Promise<FeatureSnapshot> {
    const attempts24h = this.store.saved.filter(
      (entry) =>
        entry.attempt.cardFingerprint === query.cardFingerprint &&
        entry.attempt.paymentIntentId !== query.paymentIntentId,
    ).length;

    return createFeatureSnapshot({
      cardFingerprint: query.cardFingerprint,
      attempts24h,
      declines24h: this.declinesByCard.get(query.cardFingerprint) ?? 0,
      avgCapturedCents: this.avgCapturedByCard.get(query.cardFingerprint) ?? null,
      blocked: this.blockedCards.has(query.cardFingerprint),
    });
  }
}

export class SteppingTestClock implements Clock {
  private currentMs = TEST_NOW.getTime();

  now(): Date {
    const date = new Date(this.currentMs);
    this.currentMs += SCORING_LATENCY_STEP_MS;
    return date;
  }
}

export interface InternalHttpHarnessOptions {
  testHooksEnabled?: boolean;
  ruleConfigs?: RuleConfig[];
}

export interface InternalHttpHarness {
  app: INestApplication;
  baseUrl: string;
  store: InMemoryTestAssessmentStore;
  features: InMemoryTestFeatureReader;
  close(): Promise<void>;
}

export async function startInternalHttpHarness(
  options: InternalHttpHarnessOptions = {},
): Promise<InternalHttpHarness> {
  const store = new InMemoryTestAssessmentStore();
  const features = new InMemoryTestFeatureReader(store);
  const riskConfig = testRiskConfig({ testHooksEnabled: options.testHooksEnabled ?? false });
  const rules = buildRulesFromConfig(options.ruleConfigs ?? DEFAULT_RULES);
  const useCase = new ScoreRiskUseCase(
    store,
    features,
    rules,
    DecisionThresholds.defaults(),
    new SteppingTestClock(),
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

  return {
    app,
    baseUrl,
    store,
    features,
    close: () => app.close(),
  };
}

export async function listenEphemeral(app: INestApplication): Promise<string> {
  await app.listen(0, DEFAULT_INTERNAL_HOST);
  const address = app.getHttpServer().address() as AddressInfo;
  return `http://${DEFAULT_INTERNAL_HOST}:${address.port}`;
}

export interface HttpTestResponse<T> {
  status: number;
  contentType: string | null;
  body: T;
}

export async function postRiskAssessment<T = RiskAssessmentResponseDto | ProblemDto>(
  baseUrl: string,
  body: unknown,
  authHeader: string | null = `Bearer ${TEST_SERVICE_TOKEN}`,
): Promise<HttpTestResponse<T>> {
  const headers: Record<string, string> = {
    'content-type': 'application/json',
  };
  if (authHeader !== null) {
    headers.authorization = authHeader;
  }
  const response = await fetch(`${baseUrl}/${RISK_ASSESSMENTS_PATH}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(body),
  });
  return {
    status: response.status,
    contentType: response.headers.get('content-type'),
    body: (await response.json()) as T,
  };
}
