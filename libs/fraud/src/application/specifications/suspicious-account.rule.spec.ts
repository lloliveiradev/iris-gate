import { InMemoryInvoiceHistoryRepo } from '../fakes';
import { RuleContext } from './rule';
import { SuspiciousAccountSpec } from './suspicious-account.rule';

const ctx = (isSuspicious: boolean): RuleContext => ({
  account: { ref: 'acc_1', isSuspicious },
  amountCents: 1000,
  paymentIntentId: 'pi_1',
  history: new InMemoryInvoiceHistoryRepo(),
  now: new Date('2026-05-10T12:00:00Z'),
});

describe('SuspiciousAccountSpec', () => {
  const rule = new SuspiciousAccountSpec();

  it('matches when account is suspicious', async () => {
    const out = await rule.evaluate(ctx(true));
    expect(out.matched).toBe(true);
    expect(out.baseScore).toBe(100);
    expect(out.description).toContain('acc_1');
  });

  it('does not match when account is clean', async () => {
    const out = await rule.evaluate(ctx(false));
    expect(out.matched).toBe(false);
    expect(out.baseScore).toBe(0);
  });
});
