import { createFeatureSnapshot, FeatureSnapshot } from './feature-snapshot';

/** Shared fake values for tests. Not exported from the package entry point. */
export const FIXTURE_CARD_FINGERPRINT = 'fp_test_card_001';
export const FIXTURE_OTHER_CARD_FINGERPRINT = 'fp_test_card_002';
export const FIXTURE_CUSTOMER_REF = 'cus_test_001';
export const FIXTURE_NOW = new Date('2026-05-10T12:00:00Z');

export const NORMAL_AMOUNT_CENTS = 1_000;
/** 2^31 cents: one cent above the largest 32-bit amount (21,474,836.47). */
export const ABOVE_INT32_AMOUNT_CENTS = 2_147_483_648;
export const UNSAFE_AMOUNT_CENTS = Number.MAX_SAFE_INTEGER + 1;

/** Features of a card never seen before: no attempts, declines or captures. */
export function firstTimeCardFeatures(overrides: Partial<FeatureSnapshot> = {}): FeatureSnapshot {
  return createFeatureSnapshot({
    cardFingerprint: FIXTURE_CARD_FINGERPRINT,
    attempts24h: 0,
    declines24h: 0,
    avgCapturedCents: null,
    blocked: false,
    ...overrides,
  });
}
