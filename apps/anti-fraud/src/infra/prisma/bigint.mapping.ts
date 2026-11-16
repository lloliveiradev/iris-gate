/**
 * Maps the BIGINT minor units stored in Postgres to the JavaScript numbers the
 * domain uses, and back. A value outside the safe-integer range cannot be
 * represented exactly as a number, so the mapping throws instead of rounding
 * (fail closed): an imprecise count or amount must never reach a rule.
 */

/** A stored integer does not fit a JavaScript safe integer, or is not an integer. */
export class UnsafeIntegerError extends Error {
  constructor(message: string) {
    super(message);
    this.name = new.target.name;
  }
}

const MAX_SAFE = BigInt(Number.MAX_SAFE_INTEGER);
const MIN_SAFE = BigInt(Number.MIN_SAFE_INTEGER);

/** Converts a BIGINT read from the database into a safe integer number. */
export function bigIntToSafeNumber(field: string, value: bigint | number): number {
  if (typeof value === 'number') {
    if (!Number.isSafeInteger(value)) {
      throw new UnsafeIntegerError(`${field} is not a safe integer: ${value}`);
    }
    return value;
  }
  if (value > MAX_SAFE || value < MIN_SAFE) {
    throw new UnsafeIntegerError(`${field} is outside the safe integer range: ${value}`);
  }
  return Number(value);
}

/** Converts a safe integer number into the BIGINT written to the database. */
export function safeNumberToBigInt(field: string, value: number): bigint {
  if (!Number.isSafeInteger(value)) {
    throw new UnsafeIntegerError(`${field} is not a safe integer: ${value}`);
  }
  return BigInt(value);
}
