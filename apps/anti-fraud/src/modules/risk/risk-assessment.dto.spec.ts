import { Decision, FraudReasonCode, TestScenario } from '@iris-gate/fraud';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  BLANK_RISK_STRING,
  MAX_LENGTH_RISK_STRING,
  OVERSIZED_RISK_STRING,
  TEST_AMOUNT_CENTS,
  TEST_ASSESSMENT_ID,
  TEST_CARD_BRAND,
  TEST_CARD_FINGERPRINT,
  TEST_CURRENCY,
  TEST_CUSTOMER_REF,
  TEST_MERCHANT_ID,
  TEST_PAYMENT_INTENT_ID,
  validRiskRequestDto,
} from './risk.test-fixtures';
import {
  InvalidRiskRequestError,
  parseRiskAssessmentRequest,
  toRiskAssessmentResponse,
} from './risk-assessment.dto';

describe('risk-assessment DTO mapping', () => {
  it('parses a complete request including optional customer_ref and test_scenario', () => {
    const parsed = parseRiskAssessmentRequest(
      validRiskRequestDto({
        amount_cents: ABOVE_INT32_AMOUNT_CENTS,
        test_scenario: TestScenario.REVIEW,
      }),
    );
    expect(parsed).toEqual({
      paymentIntentId: TEST_PAYMENT_INTENT_ID,
      merchantId: TEST_MERCHANT_ID,
      amountCents: ABOVE_INT32_AMOUNT_CENTS,
      currency: TEST_CURRENCY,
      cardFingerprint: TEST_CARD_FINGERPRINT,
      cardBrand: TEST_CARD_BRAND,
      customerRef: TEST_CUSTOMER_REF,
      testScenario: TestScenario.REVIEW,
    });
  });

  it('omits optional fields when not present in the wire payload', () => {
    const dto = validRiskRequestDto();
    delete dto.customer_ref;
    expect(parseRiskAssessmentRequest(dto)).toEqual({
      paymentIntentId: TEST_PAYMENT_INTENT_ID,
      merchantId: TEST_MERCHANT_ID,
      amountCents: TEST_AMOUNT_CENTS,
      currency: TEST_CURRENCY,
      cardFingerprint: TEST_CARD_FINGERPRINT,
      cardBrand: TEST_CARD_BRAND,
    });
  });

  it('accepts card_fingerprint, card_brand, and customer_ref at the 256-character upper bound', () => {
    const parsed = parseRiskAssessmentRequest(
      validRiskRequestDto({
        card_fingerprint: MAX_LENGTH_RISK_STRING,
        card_brand: MAX_LENGTH_RISK_STRING,
        customer_ref: MAX_LENGTH_RISK_STRING,
      }),
    );
    expect(parsed.cardFingerprint).toBe(MAX_LENGTH_RISK_STRING);
    expect(parsed.cardBrand).toBe(MAX_LENGTH_RISK_STRING);
    expect(parsed.customerRef).toBe(MAX_LENGTH_RISK_STRING);
  });

  it.each<[string, unknown]>([
    ['null body', null],
    ['array body', []],
    ['primitive body', 'not-an-object'],
    ['unknown property', { ...validRiskRequestDto(), extra_field: true }],
    ['non-UUID payment_intent_id', { ...validRiskRequestDto(), payment_intent_id: 'not-a-uuid' }],
    ['non-UUID merchant_id', { ...validRiskRequestDto(), merchant_id: 'not-a-uuid' }],
    ['zero amount_cents', { ...validRiskRequestDto(), amount_cents: 0 }],
    ['negative amount_cents', { ...validRiskRequestDto(), amount_cents: -100 }],
    ['fractional amount_cents', { ...validRiskRequestDto(), amount_cents: 10.5 }],
    ['lowercase currency', { ...validRiskRequestDto(), currency: 'brl' }],
    ['empty card_fingerprint', { ...validRiskRequestDto(), card_fingerprint: BLANK_RISK_STRING }],
    [
      'oversized card_fingerprint',
      { ...validRiskRequestDto(), card_fingerprint: OVERSIZED_RISK_STRING },
    ],
    ['empty card_brand', { ...validRiskRequestDto(), card_brand: '' }],
    ['oversized card_brand', { ...validRiskRequestDto(), card_brand: OVERSIZED_RISK_STRING }],
    ['non-string customer_ref', { ...validRiskRequestDto(), customer_ref: 42 }],
    ['empty customer_ref', { ...validRiskRequestDto(), customer_ref: '' }],
    ['blank customer_ref', { ...validRiskRequestDto(), customer_ref: BLANK_RISK_STRING }],
    ['oversized customer_ref', { ...validRiskRequestDto(), customer_ref: OVERSIZED_RISK_STRING }],
    ['unknown test_scenario', { ...validRiskRequestDto(), test_scenario: 'approve' }],
  ])('rejects %s with InvalidRiskRequestError', (_label, payload) => {
    expect(() => parseRiskAssessmentRequest(payload)).toThrow(InvalidRiskRequestError);
  });

  it('maps ScoreRiskOutput to RiskAssessmentResponseDto', () => {
    const dto = toRiskAssessmentResponse({
      assessmentId: TEST_ASSESSMENT_ID,
      decision: Decision.REJECT,
      score: 100,
      reasons: [
        {
          code: FraudReasonCode.BLOCKED_CARD,
          weight: 1,
          score: 100,
          description: 'card is blocked',
        },
      ],
      deduplicated: false,
      latencyMs: 12,
    });
    expect(dto).toEqual({
      assessment_id: TEST_ASSESSMENT_ID,
      decision: 'REJECT',
      score: 100,
      reasons: [
        {
          code: 'BLOCKED_CARD',
          weight: 1,
          score: 100,
          description: 'card is blocked',
        },
      ],
      latency_ms: 12,
    });
  });
});
