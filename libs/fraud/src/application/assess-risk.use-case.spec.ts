import { Decision } from '../domain/decision';
import { DecisionThresholds } from '../domain/thresholds';
import { AssessRiskUseCase } from './assess-risk.use-case';
import {
  CapturingEventPublisher,
  FixedClock,
  InMemoryAccountRepo,
  InMemoryInboxStore,
  InMemoryInvoiceHistoryRepo,
  InMemoryRiskAssessmentRepo,
  SequentialIdGen,
} from './fakes';
import { buildRulesFromConfig, DEFAULT_RULES } from './rule-registry';

const makeInput = (override: Partial<Parameters<AssessRiskUseCase['execute']>[0]> = {}) => ({
  messageId: 'msg_001',
  assessmentId: 'assess_001',
  paymentIntentId: 'pi_001',
  merchantId: 'merch_001',
  accountRef: 'acc_001',
  amountCents: 1000,
  currency: 'BRL',
  occurredAt: new Date('2026-05-10T12:00:00Z'),
  ...override,
});

const buildSut = () => {
  const inbox = new InMemoryInboxStore();
  const accounts = new InMemoryAccountRepo();
  const history = new InMemoryInvoiceHistoryRepo();
  const assessments = new InMemoryRiskAssessmentRepo();
  const publisher = new CapturingEventPublisher();
  const clock = new FixedClock(new Date('2026-05-10T12:00:01Z'));
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

  it('rejects suspicious account with high score', async () => {
    const { sut, accounts } = buildSut();
    accounts.flag('acc_001', true);
    const out = await sut.execute(makeInput());
    expect(out.decision).toBe(Decision.REJECT);
    expect(out.score).toBe(100);
    expect(out.reasons[0]?.code).toBe('SUSPICIOUS_ACCOUNT');
  });

  it('persists assessment + outbox atomically', async () => {
    const { sut, accounts, assessments } = buildSut();
    accounts.flag('acc_001', true);
    await sut.execute(makeInput());
    expect(assessments.assessments.size).toBe(1);
    expect(assessments.outbox).toHaveLength(1);
    expect(assessments.outbox[0]?.topic).toBe('fraud.assessment.completed.v1');
  });

  it('deduplicates redelivery of same messageId (Inbox)', async () => {
    const { sut, accounts, assessments } = buildSut();
    accounts.flag('acc_001', true);
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
    for (let i = 0; i < 7; i++) {
      history.add({
        id: `inv_${i}`,
        accountRef: 'acc_001',
        amountCents: 1000,
        createdAt: new Date('2026-05-10T11:30:00Z'),
      });
    }
    const out = await sut.execute(makeInput());
    expect([Decision.REVIEW, Decision.REJECT]).toContain(out.decision);
    expect(out.reasons.some((r) => r.code === 'FREQUENT_HIGH_VALUE')).toBe(true);
  });
});
