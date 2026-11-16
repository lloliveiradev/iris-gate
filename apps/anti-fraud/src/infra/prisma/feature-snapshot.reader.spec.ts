import { InvalidFeatureSnapshotError } from '@iris-gate/fraud';
import { UnsafeIntegerError } from './bigint.mapping';
import { type FeatureRow, toFeatureSnapshot } from './feature-snapshot.reader';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  ABOVE_SAFE_INTEGER,
  uniqueCardFingerprint,
} from './test-fixtures';

const CARD_FINGERPRINT = uniqueCardFingerprint();

function featureRow(overrides: Partial<FeatureRow> = {}): FeatureRow {
  return {
    attempts_24h: 0n,
    declines_24h: 0n,
    avg_captured_cents: null,
    blocked: null,
    ...overrides,
  };
}

describe('toFeatureSnapshot', () => {
  it('maps the row of a first-time card to zeros, no average and not blocked', () => {
    expect(toFeatureSnapshot(CARD_FINGERPRINT, featureRow())).toEqual({
      cardFingerprint: CARD_FINGERPRINT,
      attempts24h: 0,
      declines24h: 0,
      avgCapturedCents: null,
      blocked: false,
    });
  });

  it('maps BIGINT counts and an average above 2^31 exactly', () => {
    const snapshot = toFeatureSnapshot(
      CARD_FINGERPRINT,
      featureRow({
        attempts_24h: 6n,
        declines_24h: 4n,
        avg_captured_cents: BigInt(ABOVE_INT32_AMOUNT_CENTS),
        blocked: true,
      }),
    );
    expect(snapshot).toEqual({
      cardFingerprint: CARD_FINGERPRINT,
      attempts24h: 6,
      declines24h: 4,
      avgCapturedCents: ABOVE_INT32_AMOUNT_CENTS,
      blocked: true,
    });
    expect(Object.isFrozen(snapshot)).toBe(true);
  });

  it('reads a card profile that is not blocked as not blocked', () => {
    expect(toFeatureSnapshot(CARD_FINGERPRINT, featureRow({ blocked: false })).blocked).toBe(false);
  });

  it.each([
    ['attempts_24h', { attempts_24h: ABOVE_SAFE_INTEGER }],
    ['declines_24h', { declines_24h: ABOVE_SAFE_INTEGER }],
    ['avg_captured_cents', { avg_captured_cents: ABOVE_SAFE_INTEGER }],
  ])('fails closed when %s is outside the safe-integer range', (_column, overrides) => {
    expect(() => toFeatureSnapshot(CARD_FINGERPRINT, featureRow(overrides))).toThrow(
      UnsafeIntegerError,
    );
  });

  it('rejects a negative count instead of letting a rule silently not match', () => {
    expect(() => toFeatureSnapshot(CARD_FINGERPRINT, featureRow({ attempts_24h: -1n }))).toThrow(
      InvalidFeatureSnapshotError,
    );
  });
});
