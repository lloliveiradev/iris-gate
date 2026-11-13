import { Decision } from '../domain/decision';
import { DecisionThresholds } from '../domain/thresholds';
import { AssessRiskInput, AssessRiskUseCase } from './assess-risk.use-case';
import {
  CapturingEventPublisher,
  FixedClock,
  InMemoryAccountRepo,
  InMemoryInboxStore,
  InMemoryInvoiceHistoryRepo,
  InMemoryRiskAssessmentRepo,
  SequentialIdGen,
} from './fakes';
import { NORMAL_AMOUNT_CENTS } from './fixtures';
import { buildRulesFromConfig, DEFAULT_RULES } from './rule-registry';

const ACCOUNT_REF = 'acc_001';
const OCCURRED_AT = new Date('2026-05-10T12:00:00Z');
const NOW = new Date('2026-05-10T12:00:01Z');
const RECENT_INVOICE_AT = new Date('2026-05-10T11:30:00Z');
const OLD_INVOICE_AT = new Date('2026-05-08T12:00:00Z');
const DEFAULT_MAX_ATTEMPTS = 5;

const makeInput = (override: Partial<AssessRiskInput> = {}): AssessRiskInput => ({
  messageId: 'msg_001',
  assessmentId: 'assess_001',
  paymentIntentId: 'pi_001',
  merchantId: 'merch_001',
  accountRef: ACCOUNT_REF,
  amountCents: NORMAL_AMOUNT_CENTS,
  currency: 'BRL',
  occurredAt: OCCURRED_AT,
  ...override,
});

const addInvoices = (
  history: InMemoryInvoiceHistoryRepo,
  count: number,
  createdAt: Date,
  amountCents = NORMAL_AMOUNT_CENTS,
): void => {
  for (let i = 0; i < count; i++) {
    history.add({
      id: `inv_${createdAt.getTime()}_${i}`,
      accountRef: ACCOUNT_REF,
      amountCents,
      createdAt,
    });
  }
};

const buildSut = () => {
  const inbox = new InMemoryInboxStore();
  const accounts = new InMemoryAccountRepo();
  const history = new InMemoryInvoiceHistoryRepo();
  const assessments = new InMemoryRiskAssessmentRepo();
  const publisher = new CapturingEventPublisher();
  const clock = new FixedClock(NOW);
  const idGen = new SequentialIdGen();
  const rules = buildRulesFromConfig(DEFAULT_RULES);
  const sut = new AssessRiskUseCase(
    inbox,
    accounts,
    history,
    assessments,
    rules,
    DecisionThresholds.defaults(),
    clock,
    idGen,
  );
  return { sut, inbox, accounts, history, assessments, publisher, clock, idGen };
};

describe('AssessRiskUseCase', () => {
  it('approves clean account with normal amount', async () => {
    const { sut } = buildSut();
    const out = await sut.execute(makeInput());
    expect(out.decision).toBe(Decision.APPROVE);
    expect(out.score).toBe(0);
    expect(out.reasons).toHaveLength(0);
    expect(out.deduplicated).toBe(false);
  });

  it('rejects a suspicious account through the blocked card rule', async () => {
    const { sut, accounts } = buildSut();
    accounts.flag(ACCOUNT_REF, true);
    const out = await sut.execute(makeInput());
    expect(out.decision).toBe(Decision.REJECT);
    expect(out.score).toBe(100);
    expect(out.reasons[0]?.code).toBe('BLOCKED_CARD');
  });

  it('persists assessment + outbox atomically', async () => {
    const { sut, accounts, assessments } = buildSut();
    accounts.flag(ACCOUNT_REF, true);
    await sut.execute(makeInput());
    expect(assessments.assessments.size).toBe(1);
    expect(assessments.outbox).toHaveLength(1);
    expect(assessments.outbox[0]?.topic).toBe('fraud.assessment.completed.v1');
  });

  it('deduplicates redelivery of same messageId (Inbox)', async () => {
    const { sut, accounts, assessments } = buildSut();
    accounts.flag(ACCOUNT_REF, true);
    const first = await sut.execute(makeInput());
    expect(first.deduplicated).toBe(false);

    const second = await sut.execute(makeInput());
    expect(second.deduplicated).toBe(true);
    expect(second.decision).toBe(first.decision);
    expect(second.score).toBe(first.score);
    expect(assessments.assessments.size).toBe(1);
    expect(assessments.outbox).toHaveLength(1);
  });

  it('reviews when frequent payments push score into review band', async () => {
    const { sut, history } = buildSut();
    addInvoices(history, DEFAULT_MAX_ATTEMPTS + 2, RECENT_INVOICE_AT);
    const out = await sut.execute(makeInput());
    expect(out.decision).toBe(Decision.REVIEW);
    expect(out.reasons.map((r) => r.code)).toEqual(['VELOCITY']);
  });

  it('counts only invoices from the last 24 hours as attempts', async () => {
    const { sut, history } = buildSut();
    addInvoices(history, DEFAULT_MAX_ATTEMPTS + 2, OLD_INVOICE_AT);
    const out = await sut.execute(makeInput());
    expect(out.decision).toBe(Decision.APPROVE);
  });

  it('flags an amount far above the average of past invoices', async () => {
    const { sut, history } = buildSut();
    addInvoices(history, 1, OLD_INVOICE_AT);
    const out = await sut.execute(makeInput({ amountCents: 4 * NORMAL_AMOUNT_CENTS }));
    expect(out.decision).toBe(Decision.REVIEW);
    expect(out.reasons.map((r) => r.code)).toEqual(['UNUSUAL_AMOUNT']);
  });
});
