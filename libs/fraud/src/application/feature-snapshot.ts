import { InvalidFeatureSnapshotError } from '../domain/errors';

/**
 * What the rules know about a card when a payment is scored, read in one call.
 * Counts cover the 24 hours before the attempt being scored. Amounts are integer
 * minor units. Build it with `createFeatureSnapshot`, which validates and freezes it.
 */
export interface FeatureSnapshot {
  readonly cardFingerprint: string;
  readonly attempts24h: number;
  readonly declines24h: number;
  /** Average captured amount for the card, or `null` when nothing was captured yet. */
  readonly avgCapturedCents: number | null;
  readonly blocked: boolean;
}

/** Identifies whose features to load. */
export interface FeatureSnapshotQuery {
  cardFingerprint: string;
  customerRef?: string;
}

/** Port that loads the features of one card (and optional customer) in a single call. */
export interface FeatureSnapshotReader {
  load(query: FeatureSnapshotQuery): Promise<FeatureSnapshot>;
}

/**
 * Validates a snapshot and returns a frozen copy. A malformed snapshot throws
 * instead of letting a rule silently not match (fail closed).
 */
export function createFeatureSnapshot(input: FeatureSnapshot): FeatureSnapshot {
  if (typeof input.cardFingerprint !== 'string' || input.cardFingerprint.length === 0) {
    throw new InvalidFeatureSnapshotError('cardFingerprint is required');
  }
  assertCount('attempts24h', input.attempts24h);
  assertCount('declines24h', input.declines24h);
  if (input.avgCapturedCents !== null) {
    assertCount('avgCapturedCents', input.avgCapturedCents);
  }
  if (typeof input.blocked !== 'boolean') {
    throw new InvalidFeatureSnapshotError(
      `blocked must be a boolean, got ${String(input.blocked)}`,
    );
  }
  return Object.freeze({
    cardFingerprint: input.cardFingerprint,
    attempts24h: input.attempts24h,
    declines24h: input.declines24h,
    avgCapturedCents: input.avgCapturedCents,
    blocked: input.blocked,
  });
}

function assertCount(field: string, value: number): void {
  if (!Number.isSafeInteger(value) || value < 0) {
    throw new InvalidFeatureSnapshotError(
      `${field} must be a non-negative safe integer, got ${value}`,
    );
  }
}
