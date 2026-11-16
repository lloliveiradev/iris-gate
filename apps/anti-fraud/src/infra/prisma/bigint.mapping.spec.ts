import { bigIntToSafeNumber, safeNumberToBigInt, UnsafeIntegerError } from './bigint.mapping';
import { ABOVE_INT32_AMOUNT_CENTS, ABOVE_SAFE_INTEGER } from './test-fixtures';

const FIELD = 'amountCents';

describe('bigIntToSafeNumber', () => {
  it.each([
    ['zero', 0n, 0],
    ['an amount above 2^31', BigInt(ABOVE_INT32_AMOUNT_CENTS), ABOVE_INT32_AMOUNT_CENTS],
    ['the largest safe integer', BigInt(Number.MAX_SAFE_INTEGER), Number.MAX_SAFE_INTEGER],
    ['the smallest safe integer', BigInt(Number.MIN_SAFE_INTEGER), Number.MIN_SAFE_INTEGER],
    ['a number the driver already decoded', ABOVE_INT32_AMOUNT_CENTS, ABOVE_INT32_AMOUNT_CENTS],
  ])('maps %s exactly', (_case, value, expected) => {
    expect(bigIntToSafeNumber(FIELD, value)).toBe(expected);
  });

  it.each([
    ['above the safe range', ABOVE_SAFE_INTEGER],
    ['below the safe range', -ABOVE_SAFE_INTEGER],
    ['an unsafe number', Number.MAX_SAFE_INTEGER + 1],
    ['a fraction', 1.5],
  ])('throws on %s instead of rounding', (_case, value) => {
    expect(() => bigIntToSafeNumber(FIELD, value)).toThrow(UnsafeIntegerError);
  });

  it('names the field in the error', () => {
    expect(() => bigIntToSafeNumber(FIELD, ABOVE_SAFE_INTEGER)).toThrow(FIELD);
  });
});

describe('safeNumberToBigInt', () => {
  it('maps an amount above 2^31 exactly', () => {
    expect(safeNumberToBigInt(FIELD, ABOVE_INT32_AMOUNT_CENTS)).toBe(
      BigInt(ABOVE_INT32_AMOUNT_CENTS),
    );
  });

  it.each([
    ['an unsafe number', Number.MAX_SAFE_INTEGER + 1],
    ['a fraction', 1.5],
    ['NaN', Number.NaN],
  ])('throws on %s', (_case, value) => {
    expect(() => safeNumberToBigInt(FIELD, value)).toThrow(UnsafeIntegerError);
  });
});
