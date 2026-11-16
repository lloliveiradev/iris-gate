import { randomUUID } from 'node:crypto';
import type { Clock, FeatureSnapshotQuery } from '@iris-gate/fraud';
import { UnsafeIntegerError } from './bigint.mapping';
import { FEATURE_WINDOW_MS, PrismaFeatureSnapshotReader } from './feature-snapshot.reader';
import { PaymentOutcomeKind } from './generated/client';
import type { PrismaService } from './prisma.service';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  ABOVE_SAFE_INTEGER,
  connectTestDatabase,
  TEST_AMOUNT_CENTS,
  TEST_CARD_BRAND,
  TEST_CURRENCY,
  TEST_DATABASE_URL,
  TEST_MERCHANT_ID,
  TEST_NOW,
  uniqueCardFingerprint,
} from './test-fixtures';

const ONE_MS = 1;
const WINDOW_START = new Date(TEST_NOW.getTime() - FEATURE_WINDOW_MS);
const OLDER_THAN_WINDOW = new Date(WINDOW_START.getTime() - FEATURE_WINDOW_MS);
const fixedClock: Clock = { now: () => TEST_NOW };

interface OutcomeRow {
  cardFingerprint: string;
  occurredAt: Date;
  paymentIntentId?: string;
  amountCents?: bigint;
}

const describeWithDatabase = TEST_DATABASE_URL ? describe : describe.skip;

describeWithDatabase('PrismaFeatureSnapshotReader (Postgres)', () => {
  let prisma: PrismaService;
  let reader: PrismaFeatureSnapshotReader;
  const cardsUsed = new Set<string>();

  beforeAll(async () => {
    prisma = connectTestDatabase(TEST_DATABASE_URL ?? '');
    await prisma.$connect();
    reader = new PrismaFeatureSnapshotReader(prisma, fixedClock);
  });

  afterAll(async () => {
    const cardFingerprint = { in: [...cardsUsed] };
    await prisma.paymentOutcome.deleteMany({ where: { cardFingerprint } });
    await prisma.cardProfile.deleteMany({ where: { cardFingerprint } });
    await prisma.$disconnect();
  });

  function newCard(): string {
    const card = uniqueCardFingerprint();
    cardsUsed.add(card);
    return card;
  }

  async function record(outcome: PaymentOutcomeKind, row: OutcomeRow): Promise<void> {
    cardsUsed.add(row.cardFingerprint);
    await prisma.paymentOutcome.create({
      data: {
        paymentIntentId: row.paymentIntentId ?? randomUUID(),
        outcome,
        merchantId: TEST_MERCHANT_ID,
        cardFingerprint: row.cardFingerprint,
        cardBrand: TEST_CARD_BRAND,
        amountCents: row.amountCents ?? BigInt(TEST_AMOUNT_CENTS),
        currency: TEST_CURRENCY,
        occurredAt: row.occurredAt,
      },
    });
  }

  function queryFor(cardFingerprint: string, paymentIntentId = randomUUID()): FeatureSnapshotQuery {
    return { cardFingerprint, paymentIntentId };
  }

  it("reads a first-time card as zeros, no average and not blocked, ignoring other cards' rows", async () => {
    const card = newCard();
    const otherCard = newCard();
    await record(PaymentOutcomeKind.attempted, {
      cardFingerprint: otherCard,
      occurredAt: WINDOW_START,
    });
    await record(PaymentOutcomeKind.declined, {
      cardFingerprint: otherCard,
      occurredAt: WINDOW_START,
    });
    await record(PaymentOutcomeKind.captured, {
      cardFingerprint: otherCard,
      occurredAt: WINDOW_START,
    });
    await prisma.cardProfile.create({ data: { cardFingerprint: otherCard, blocked: true } });

    await expect(reader.load(queryFor(card))).resolves.toEqual({
      cardFingerprint: card,
      attempts24h: 0,
      declines24h: 0,
      avgCapturedCents: null,
      blocked: false,
    });
  });

  it.each([
    [PaymentOutcomeKind.attempted, 'attempts24h'],
    [PaymentOutcomeKind.declined, 'declines24h'],
  ] as const)('counts %s outcomes in [now - 24h, now)', async (outcome, feature) => {
    const card = newCard();
    const inside = [WINDOW_START, new Date(TEST_NOW.getTime() - ONE_MS)];
    const outside = [new Date(WINDOW_START.getTime() - ONE_MS), TEST_NOW];
    for (const occurredAt of [...inside, ...outside]) {
      await record(outcome, { cardFingerprint: card, occurredAt });
    }

    const snapshot = await reader.load(queryFor(card));

    expect(snapshot[feature]).toBe(inside.length);
  });

  it('leaves out every outcome of the payment intent being scored', async () => {
    const card = newCard();
    const inFlightIntentId = randomUUID();
    const recent = new Date(TEST_NOW.getTime() - ONE_MS);
    await record(PaymentOutcomeKind.attempted, {
      cardFingerprint: card,
      occurredAt: recent,
      paymentIntentId: inFlightIntentId,
    });
    await record(PaymentOutcomeKind.declined, {
      cardFingerprint: card,
      occurredAt: recent,
      paymentIntentId: inFlightIntentId,
    });
    await record(PaymentOutcomeKind.captured, {
      cardFingerprint: card,
      occurredAt: recent,
      paymentIntentId: inFlightIntentId,
    });
    await record(PaymentOutcomeKind.attempted, { cardFingerprint: card, occurredAt: recent });

    await expect(reader.load(queryFor(card, inFlightIntentId))).resolves.toMatchObject({
      attempts24h: 1,
      declines24h: 0,
      avgCapturedCents: null,
    });
  });

  it('floors the average of captured amounts of any age, ignoring attempts and declines', async () => {
    const card = newCard();
    await record(PaymentOutcomeKind.captured, {
      cardFingerprint: card,
      occurredAt: OLDER_THAN_WINDOW,
      amountCents: BigInt(TEST_AMOUNT_CENTS),
    });
    await record(PaymentOutcomeKind.captured, {
      cardFingerprint: card,
      occurredAt: WINDOW_START,
      amountCents: BigInt(TEST_AMOUNT_CENTS + 1),
    });
    await record(PaymentOutcomeKind.attempted, {
      cardFingerprint: card,
      occurredAt: WINDOW_START,
      amountCents: BigInt(ABOVE_INT32_AMOUNT_CENTS),
    });

    const snapshot = await reader.load(queryFor(card));

    expect(snapshot.avgCapturedCents).toBe(TEST_AMOUNT_CENTS);
  });

  it('stores a captured amount above 2^31 cents and reads it back exactly (AC3)', async () => {
    const card = newCard();
    await record(PaymentOutcomeKind.captured, {
      cardFingerprint: card,
      occurredAt: WINDOW_START,
      amountCents: BigInt(ABOVE_INT32_AMOUNT_CENTS),
    });

    const stored = await prisma.paymentOutcome.findFirstOrThrow({
      where: { cardFingerprint: card },
    });
    const snapshot = await reader.load(queryFor(card));

    expect(stored.amountCents).toBe(BigInt(ABOVE_INT32_AMOUNT_CENTS));
    expect(snapshot.avgCapturedCents).toBe(ABOVE_INT32_AMOUNT_CENTS);
  });

  it.each([true, false])('reads blocked = %s from the card profile', async (blocked) => {
    const card = newCard();
    await prisma.cardProfile.create({ data: { cardFingerprint: card, blocked } });

    const snapshot = await reader.load(queryFor(card));

    expect(snapshot.blocked).toBe(blocked);
  });

  it('fails closed when a stored amount is outside the safe-integer range', async () => {
    const card = newCard();
    await record(PaymentOutcomeKind.captured, {
      cardFingerprint: card,
      occurredAt: WINDOW_START,
      amountCents: ABOVE_SAFE_INTEGER,
    });

    await expect(reader.load(queryFor(card))).rejects.toThrow(UnsafeIntegerError);
  });
});
