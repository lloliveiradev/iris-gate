import { type ScoreRiskInput, type ScoreRiskOutput, TestScenario } from '@iris-gate/fraud';
import type { components } from '../../../../../libs/contracts/generated/ts/fraud-internal';

export type RiskAssessmentRequestDto = components['schemas']['RiskAssessmentRequest'];
export type RiskAssessmentResponseDto = components['schemas']['RiskAssessmentResponse'];
export type RiskReasonDto = components['schemas']['RiskReason'];
export type ProblemDto = components['schemas']['Problem'];

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const CURRENCY_PATTERN = /^[A-Z]{3}$/;
export const MAX_RISK_STRING_LENGTH = 256;

const ALLOWED_REQUEST_KEYS = new Set<string>([
  'payment_intent_id',
  'merchant_id',
  'amount_cents',
  'currency',
  'card_fingerprint',
  'card_brand',
  'customer_ref',
  'test_scenario',
]);

/** The request body does not match `RiskAssessmentRequest` from `fraud-internal.openapi.yaml`. */
export class InvalidRiskRequestError extends Error {
  constructor(message: string) {
    super(message);
    this.name = new.target.name;
  }
}

/**
 * Validates an untrusted request body against the `RiskAssessmentRequest`
 * OpenAPI schema and converts it to `ScoreRiskInput`.
 */
export function parseRiskAssessmentRequest(body: unknown): ScoreRiskInput {
  if (typeof body !== 'object' || body === null || Array.isArray(body)) {
    throw new InvalidRiskRequestError('request body must be a JSON object');
  }

  const record = body as Record<string, unknown>;
  for (const key of Object.keys(record)) {
    if (!ALLOWED_REQUEST_KEYS.has(key)) {
      throw new InvalidRiskRequestError(`unknown field: ${key}`);
    }
  }

  const paymentIntentId = requireUuid('payment_intent_id', record.payment_intent_id);
  const merchantId = requireUuid('merchant_id', record.merchant_id);
  const amountCents = requireAmountCents(record.amount_cents);
  const currency = requireCurrency(record.currency);
  const cardFingerprint = requireNonEmptyString('card_fingerprint', record.card_fingerprint);
  const cardBrand = requireNonEmptyString('card_brand', record.card_brand);
  const customerRef = optionalString('customer_ref', record.customer_ref);
  const testScenario = optionalTestScenario(record.test_scenario);

  const dto: RiskAssessmentRequestDto = {
    payment_intent_id: paymentIntentId,
    merchant_id: merchantId,
    amount_cents: amountCents,
    currency,
    card_fingerprint: cardFingerprint,
    card_brand: cardBrand,
    ...(customerRef !== undefined ? { customer_ref: customerRef } : {}),
    ...(testScenario !== undefined ? { test_scenario: testScenario } : {}),
  };

  return toScoreRiskInput(dto);
}

/** Maps a validated `RiskAssessmentRequestDto` to `ScoreRiskInput`. */
export function toScoreRiskInput(dto: RiskAssessmentRequestDto): ScoreRiskInput {
  return {
    paymentIntentId: dto.payment_intent_id,
    merchantId: dto.merchant_id,
    amountCents: dto.amount_cents,
    currency: dto.currency,
    cardFingerprint: dto.card_fingerprint,
    cardBrand: dto.card_brand,
    ...(dto.customer_ref !== undefined ? { customerRef: dto.customer_ref } : {}),
    ...(dto.test_scenario !== undefined ? { testScenario: dto.test_scenario } : {}),
  };
}

/** Maps `ScoreRiskOutput` to the `RiskAssessmentResponse` OpenAPI contract. */
export function toRiskAssessmentResponse(output: ScoreRiskOutput): RiskAssessmentResponseDto {
  return {
    assessment_id: output.assessmentId,
    decision: output.decision,
    score: output.score,
    reasons: output.reasons.map((reason): RiskReasonDto => ({
      code: reason.code,
      weight: reason.weight,
      score: reason.score,
      description: reason.description,
    })),
    latency_ms: output.latencyMs,
  };
}

function requireUuid(field: string, value: unknown): string {
  if (typeof value !== 'string' || !UUID_PATTERN.test(value)) {
    throw new InvalidRiskRequestError(`${field} must be a valid UUID`);
  }
  return value;
}

function requireAmountCents(value: unknown): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 1) {
    throw new InvalidRiskRequestError('amount_cents must be a positive safe integer');
  }
  return value;
}

function requireCurrency(value: unknown): string {
  if (typeof value !== 'string' || !CURRENCY_PATTERN.test(value)) {
    throw new InvalidRiskRequestError('currency must be an ISO-4217 3-letter uppercase code');
  }
  return value;
}

function requireNonEmptyString(field: string, value: unknown): string {
  if (typeof value !== 'string' || value.trim().length === 0) {
    throw new InvalidRiskRequestError(`${field} must be a non-empty string`);
  }
  if (value.length > MAX_RISK_STRING_LENGTH) {
    throw new InvalidRiskRequestError(
      `${field} must be at most ${MAX_RISK_STRING_LENGTH} characters`,
    );
  }
  return value;
}

function optionalString(field: string, value: unknown): string | undefined {
  if (value === undefined) return undefined;
  return requireNonEmptyString(field, value);
}

function optionalTestScenario(value: unknown): TestScenario | undefined {
  if (value === undefined) return undefined;
  if (value !== TestScenario.BLOCKED && value !== TestScenario.REVIEW) {
    throw new InvalidRiskRequestError('test_scenario must be "blocked" or "review"');
  }
  return value;
}
