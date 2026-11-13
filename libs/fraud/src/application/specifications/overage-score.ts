const OVERAGE_BASE_SCORE = 60;
const OVERAGE_STEP_SCORE = 10;
const MAX_BASE_SCORE = 100;

/**
 * Base score for a count that went over its limit: 60 for the first unit over,
 * plus 10 per extra unit, capped at 100.
 */
export function overageScore(overage: number): number {
  return Math.min(MAX_BASE_SCORE, OVERAGE_BASE_SCORE + overage * OVERAGE_STEP_SCORE);
}
