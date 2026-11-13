/**
 * Reason codes attached to a fraud decision. The set is shared with the internal
 * risk-assessment contract: `BLOCKED_CARD`, `UNUSUAL_AMOUNT`, `VELOCITY` and
 * `DECLINE_BURST` come from rules, `TEST_SCENARIO` from the test-hook override,
 * and `RISK_UNAVAILABLE` from callers that fail closed when scoring is unreachable.
 */
export const FraudReasonCode = {
  BLOCKED_CARD: 'BLOCKED_CARD',
  UNUSUAL_AMOUNT: 'UNUSUAL_AMOUNT',
  VELOCITY: 'VELOCITY',
  DECLINE_BURST: 'DECLINE_BURST',
  TEST_SCENARIO: 'TEST_SCENARIO',
  RISK_UNAVAILABLE: 'RISK_UNAVAILABLE',
} as const;

export type FraudReasonCode = (typeof FraudReasonCode)[keyof typeof FraudReasonCode];

export interface FraudReason {
  readonly code: FraudReasonCode;
  readonly weight: number;
  readonly score: number;
  readonly description: string;
}
