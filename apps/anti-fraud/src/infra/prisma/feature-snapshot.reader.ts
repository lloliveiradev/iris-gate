import {
  type Clock,
  createFeatureSnapshot,
  type FeatureSnapshot,
  type FeatureSnapshotQuery,
  type FeatureSnapshotReader,
} from '@iris-gate/fraud';
import { bigIntToSafeNumber } from './bigint.mapping';
import { PaymentOutcomeKind, type PrismaClient } from './generated/client';

/** Length of the trailing window `attempts24h` and `declines24h` count over. */
export const FEATURE_WINDOW_MS = 24 * 60 * 60 * 1000;

/** The single row the feature query returns; Postgres BIGINT arrives as `bigint`. */
export interface FeatureRow {
  attempts_24h: bigint | number;
  declines_24h: bigint | number;
  avg_captured_cents: bigint | number | null;
  blocked: boolean | null;
}

/**
 * Loads the `FeatureSnapshot` of a card from `payment_outcomes` and `card_profiles`
 * in ONE query, as the `FeatureSnapshotReader` contract defines it:
 *
 * - `attempts24h` and `declines24h` count `attempted` and `declined` outcomes whose
 *   `occurred_at` is in `[now - 24h, now)`: a row exactly 24 h old counts, a row at
 *   `now` does not;
 * - `avgCapturedCents` is `FLOOR(AVG(amount_cents))` over every `captured` outcome,
 *   or `null` when the card has none;
 * - `blocked` is `card_profiles.blocked`; a card without a profile is not blocked;
 * - nothing recorded for the payment intent being scored counts, so the in-flight
 *   attempt is excluded even when its `attempted` row is already visible.
 *
 * `now` comes from the clock. `customerRef` is not read: the snapshot carries no
 * customer feature yet. A BIGINT outside the safe-integer range throws
 * `UnsafeIntegerError` (fail closed).
 */
export class PrismaFeatureSnapshotReader implements FeatureSnapshotReader {
  constructor(
    private readonly prisma: PrismaClient,
    private readonly clock: Clock,
  ) {}

  async load(query: FeatureSnapshotQuery): Promise<FeatureSnapshot> {
    const now = this.clock.now();
    const windowStart = new Date(now.getTime() - FEATURE_WINDOW_MS).toISOString();
    const windowEnd = now.toISOString();
    // Timestamps travel as ISO-8601 strings with a `Z` offset, so the comparison
    // does not depend on the session time zone.
    const rows = await this.prisma.$queryRaw<FeatureRow[]>`
      SELECT
        COUNT(*) FILTER (
          WHERE o.outcome = ${PaymentOutcomeKind.attempted}::payment_outcome_kind
            AND o.occurred_at >= ${windowStart}::timestamptz
            AND o.occurred_at < ${windowEnd}::timestamptz
        ) AS attempts_24h,
        COUNT(*) FILTER (
          WHERE o.outcome = ${PaymentOutcomeKind.declined}::payment_outcome_kind
            AND o.occurred_at >= ${windowStart}::timestamptz
            AND o.occurred_at < ${windowEnd}::timestamptz
        ) AS declines_24h,
        FLOOR(
          AVG(o.amount_cents) FILTER (
            WHERE o.outcome = ${PaymentOutcomeKind.captured}::payment_outcome_kind
          )
        )::bigint AS avg_captured_cents,
        (
          SELECT p.blocked FROM card_profiles p
          WHERE p.card_fingerprint = ${query.cardFingerprint}
        ) AS blocked
      FROM payment_outcomes o
      WHERE o.card_fingerprint = ${query.cardFingerprint}
        AND o.payment_intent_id <> ${query.paymentIntentId}
    `;
    const [row] = rows;
    if (!row) {
      // An aggregate without GROUP BY always returns one row; anything else is a bug.
      throw new Error('feature query returned no row');
    }
    return toFeatureSnapshot(query.cardFingerprint, row);
  }
}

/** Maps the feature row to a validated snapshot; BIGINTs must be safe integers. */
export function toFeatureSnapshot(cardFingerprint: string, row: FeatureRow): FeatureSnapshot {
  return createFeatureSnapshot({
    cardFingerprint,
    attempts24h: bigIntToSafeNumber('attempts24h', row.attempts_24h),
    declines24h: bigIntToSafeNumber('declines24h', row.declines_24h),
    avgCapturedCents:
      row.avg_captured_cents === null
        ? null
        : bigIntToSafeNumber('avgCapturedCents', row.avg_captured_cents),
    blocked: row.blocked === true,
  });
}
