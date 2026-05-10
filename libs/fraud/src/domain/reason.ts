export const FraudReasonCode = {
  SUSPICIOUS_ACCOUNT: 'SUSPICIOUS_ACCOUNT',
  UNUSUAL_AMOUNT: 'UNUSUAL_AMOUNT',
  FREQUENT_HIGH_VALUE: 'FREQUENT_HIGH_VALUE',
} as const;

export type FraudReasonCode = (typeof FraudReasonCode)[keyof typeof FraudReasonCode];

export interface FraudReason {
  readonly code: FraudReasonCode;
  readonly weight: number;
  readonly score: number;
  readonly description: string;
}
