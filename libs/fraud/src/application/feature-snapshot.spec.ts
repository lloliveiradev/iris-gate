import { InvalidFeatureSnapshotError } from '../domain/errors';
import { InMemoryFeatureSnapshotReader } from './fakes';
import { createFeatureSnapshot, FeatureSnapshot } from './feature-snapshot';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  FIXTURE_CARD_FINGERPRINT,
  FIXTURE_CUSTOMER_REF,
  FIXTURE_OTHER_CARD_FINGERPRINT,
  firstTimeCardFeatures,
  UNSAFE_AMOUNT_CENTS,
} from './fixtures';

describe('createFeatureSnapshot', () => {
  it('returns a frozen copy', () => {
    const input = { ...firstTimeCardFeatures(), attempts24h: 2 };
    const snapshot = createFeatureSnapshot(input);
    expect(snapshot).toEqual(input);
    expect(snapshot).not.toBe(input);
    expect(Object.isFrozen(snapshot)).toBe(true);
  });

  it('accepts an average above the 32-bit range', () => {
    const snapshot = firstTimeCardFeatures({ avgCapturedCents: ABOVE_INT32_AMOUNT_CENTS });
    expect(snapshot.avgCapturedCents).toBe(ABOVE_INT32_AMOUNT_CENTS);
  });

  it.each<[string, Partial<FeatureSnapshot>]>([
    ['empty fingerprint', { cardFingerprint: '' }],
    ['negative attempts', { attempts24h: -1 }],
    ['fractional attempts', { attempts24h: 1.5 }],
    ['NaN attempts', { attempts24h: Number.NaN }],
    ['negative declines', { declines24h: -1 }],
    ['infinite declines', { declines24h: Number.POSITIVE_INFINITY }],
    ['fractional average', { avgCapturedCents: 10.5 }],
    ['unsafe average', { avgCapturedCents: UNSAFE_AMOUNT_CENTS }],
    ['missing blocked flag', { blocked: undefined as unknown as boolean }],
  ])('rejects %s', (_label, override) => {
    const base: FeatureSnapshot = {
      cardFingerprint: FIXTURE_CARD_FINGERPRINT,
      attempts24h: 0,
      declines24h: 0,
      avgCapturedCents: null,
      blocked: false,
    };
    expect(() => createFeatureSnapshot({ ...base, ...override })).toThrow(
      InvalidFeatureSnapshotError,
    );
  });
});

describe('InMemoryFeatureSnapshotReader', () => {
  it('returns the stored snapshot for a known card', async () => {
    const reader = new InMemoryFeatureSnapshotReader();
    const stored = firstTimeCardFeatures({ attempts24h: 3, blocked: true });
    reader.set(stored);

    const loaded = await reader.load({
      cardFingerprint: FIXTURE_CARD_FINGERPRINT,
      customerRef: FIXTURE_CUSTOMER_REF,
    });

    expect(loaded).toEqual(stored);
    expect(reader.queries).toEqual([
      { cardFingerprint: FIXTURE_CARD_FINGERPRINT, customerRef: FIXTURE_CUSTOMER_REF },
    ]);
  });

  it('reads an unknown card as a first-time card', async () => {
    const reader = new InMemoryFeatureSnapshotReader();
    const loaded = await reader.load({ cardFingerprint: FIXTURE_OTHER_CARD_FINGERPRINT });
    expect(loaded).toEqual({
      cardFingerprint: FIXTURE_OTHER_CARD_FINGERPRINT,
      attempts24h: 0,
      declines24h: 0,
      avgCapturedCents: null,
      blocked: false,
    });
  });
});
