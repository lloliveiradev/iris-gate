import { randomUUID } from 'node:crypto';
import { PrismaService } from './prisma.service';

/**
 * Shared fake values and helpers for the Prisma adapter specs. Nothing outside a
 * spec imports this file.
 */

/**
 * The database the integration specs (`*.integration.spec.ts`) write to, from
 * `ANTIFRAUD_TEST_DATABASE_URL`: a disposable database with the migrations applied
 * (`make -C apps/anti-fraud test-int` applies them). The specs never read
 * `DATABASE_URL`, so they cannot write to the development database. When it is
 * unset, the integration specs are skipped.
 */
export const TEST_DATABASE_URL = process.env.ANTIFRAUD_TEST_DATABASE_URL;

export const TEST_NOW = new Date('2026-11-14T12:00:00.000Z');
export const TEST_MERCHANT_ID = '00000000-0000-4000-8000-0000000000d1';
export const TEST_CARD_BRAND = 'visa';
export const TEST_CURRENCY = 'BRL';
export const TEST_AMOUNT_CENTS = 1_000;
/** 2^31 cents: one cent above the largest 32-bit amount (21,474,836.47). */
export const ABOVE_INT32_AMOUNT_CENTS = 2_147_483_648;
/** The first integer a JavaScript number cannot hold exactly. */
export const ABOVE_SAFE_INTEGER = BigInt(Number.MAX_SAFE_INTEGER) + 1n;

/** A card fingerprint no earlier run used, so specs share a database without cleanup races. */
export function uniqueCardFingerprint(): string {
  return `fp_test_${randomUUID()}`;
}

/** A Prisma client on the integration test database. */
export function connectTestDatabase(url: string): PrismaService {
  return new PrismaService({ url });
}
