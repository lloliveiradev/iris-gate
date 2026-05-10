import { Decision } from './decision';
import {
  AssessmentAlreadyCompletedError,
  InvalidAssessmentInputError,
} from './errors';
import { FraudReasonCode, FraudReason } from './reason';
import { AssessmentStatus, RiskAssessment } from './risk-assessment';

const baseInput = () => ({
  id: '11111111-1111-1111-1111-111111111111',
  paymentIntentId: '22222222-2222-2222-2222-222222222222',
  merchantId: '33333333-3333-3333-3333-333333333333',
  accountRef: 'acc_abc',
  amountCents: 1500,
  currency: 'BRL',
  requestedAt: new Date('2026-05-10T12:00:00Z'),
});

describe('RiskAssessment', () => {
  it('creates a pending assessment with valid input', () => {
    const a = RiskAssessment.create(baseInput());
    expect(a.status).toBe(AssessmentStatus.PENDING);
    expect(a.decision).toBeNull();
    expect(a.score).toBe(0);
    expect(a.reasons).toHaveLength(0);
  });

  it.each([
    ['empty id', { id: '' }],
    ['empty merchantId', { merchantId: '' }],
    ['negative amount', { amountCents: -1 }],
    ['fractional amount', { amountCents: 1.5 }],
    ['zero amount', { amountCents: 0 }],
    ['lowercase currency', { currency: 'brl' }],
    ['short currency', { currency: 'BR' }],
  ])('rejects invalid input: %s', (_label, override) => {
    expect(() => RiskAssessment.create({ ...baseInput(), ...override })).toThrow(
      InvalidAssessmentInputError,
    );
  });

  it('completes with decision/score/reasons', () => {
    const a = RiskAssessment.create(baseInput());
    const reasons: FraudReason[] = [
      { code: FraudReasonCode.SUSPICIOUS_ACCOUNT, weight: 1, score: 100, description: 'flag' },
    ];
    const completedAt = new Date('2026-05-10T12:00:01Z');
    a.complete(Decision.REJECT, 100, reasons, completedAt);
    expect(a.status).toBe(AssessmentStatus.COMPLETED);
    expect(a.decision).toBe(Decision.REJECT);
    expect(a.score).toBe(100);
    expect(a.reasons).toEqual(reasons);
    expect(a.completedAt).toEqual(completedAt);
  });

  it('forbids completing twice', () => {
    const a = RiskAssessment.create(baseInput());
    a.complete(Decision.APPROVE, 0, [], new Date());
    expect(() => a.complete(Decision.APPROVE, 0, [], new Date())).toThrow(
      AssessmentAlreadyCompletedError,
    );
  });

  it('fail() transitions to FAILED only from PENDING', () => {
    const a = RiskAssessment.create(baseInput());
    a.fail(new Date());
    expect(a.status).toBe(AssessmentStatus.FAILED);
    expect(() => a.fail(new Date())).toThrow(AssessmentAlreadyCompletedError);
  });

  it('roundtrips via snapshot', () => {
    const a = RiskAssessment.create(baseInput());
    a.complete(
      Decision.REVIEW,
      45,
      [{ code: FraudReasonCode.UNUSUAL_AMOUNT, weight: 0.6, score: 75, description: 'spike' }],
      new Date('2026-05-10T12:01:00Z'),
    );
    const snap = a.toSnapshot();
    const b = RiskAssessment.rehydrate(snap);
    expect(b.toSnapshot()).toEqual(snap);
  });
});
