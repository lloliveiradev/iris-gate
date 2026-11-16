import { InvalidFeatureSnapshotError } from '../domain/errors';

/**
 * What the rules know about a card when a payment is scored, read in one call.
 * Both `attempts24h` and `declines24h` cover the trailing 24 hours BEFORE the attempt
 * being scored and EXCLUDE the current in-flight attempt (`0` for a first-time card).
 * Amounts are integer minor units. Build it with `createFeatureSnapshot`, which
 * validates and freezes it.
 */
export interface FeatureSnapshot {
  readonly cardFingerprint: string;
  /**
   * Number of prior payment attempts for the card in the trailing 24 hours, excluding
   * the attempt currently being scored (`0` for a first-time card). Counting the
   * current attempt, this request is attempt number `attempts24h + 1` in the window,
   * so `VelocitySpec` triggers when `attempts24h + 1 > maxAttempts24h`.
   */
  readonly attempts24h: number;
  /**
   * Number of prior declined attempts for the card in the trailing 24 hours, excluding
   * the attempt currently being scored (which has not reached the acquirer yet; `0`
   * for a first-time card). `DeclineBurstSpec` triggers when `declines24h > maxDeclines24h`.
   */
  readonly declines24h: number;
  /**
   * Average captured amount for the card in integer minor units, or `null` when nothing
   * was captured yet. It must be an integer: a reader backed by SQL `AVG` (which returns
   * a fraction) floors the result before returning it; `createFeatureSnapshot` rejects
   * a fractional value.
   */
  readonly avgCapturedCents: number | null;
  readonly blocked: boolean;
}

/** Identifies whose features to load, and which in-flight attempt to leave out. */
export interface FeatureSnapshotQuery {
  cardFingerprint: string;
  customerRef?: string;
  /**
   * The payment intent being scored. Nothing recorded for it counts in the snapshot,
   * so a reader excludes the in-flight attempt even if its `attempted` row is visible.
   */
  paymentIntentId: string;
}

/**
 * Port that loads the features of one card (and optional customer) in a single call.
 * Every count and amount it returns is an integer; an adapter computing
 * `avgCapturedCents` with SQL `AVG` floors it (for example `FLOOR(AVG(amount_cents))`).
 */
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
