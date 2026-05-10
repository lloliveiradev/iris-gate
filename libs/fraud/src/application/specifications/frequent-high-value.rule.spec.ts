import { InMemoryInvoiceHistoryRepo } from '../fakes';
import { FrequentHighValueSpec } from './frequent-high-value.rule';
import { RuleContext } from './rule';

const buildCtx = (history: InMemoryInvoiceHistoryRepo): RuleContext => ({
  account: { ref: 'acc_1', isSuspicious: false },
  amountCents: 1000,
  paymentIntentId: 'pi_1',
  history,
  now: new Date('2026-05-10T12:00:00Z'),
});

describe('FrequentHighValueSpec', () => {
  const rule = new FrequentHighValueSpec({ maxRecentCount: 3, windowHours: 1 });

  it('does not match with few recent invoices', async () => {
    const history = new InMemoryInvoiceHistoryRepo();
    for (let i = 0; i < 3; i++) {
      history.add({
        id: `inv_${i}`,
        accountRef: 'acc_1',
        amountCents: 100,
        createdAt: new Date('2026-05-10T11:30:00Z'),
      });
    }
    const out = await rule.evaluate(buildCtx(history));
    expect(out.matched).toBe(false);
  });

  it('matches when invoice count exceeds limit in window', async () => {
    const history = new InMemoryInvoiceHistoryRepo();
    for (let i = 0; i < 5; i++) {
      history.add({
        id: `inv_${i}`,
        accountRef: 'acc_1',
        amountCents: 100,
        createdAt: new Date('2026-05-10T11:30:00Z'),
      });
    }
    const out = await rule.evaluate(buildCtx(history));
    expect(out.matched).toBe(true);
    expect(out.baseScore).toBeGreaterThanOrEqual(60);
    expect(out.description).toContain('5 payments');
  });

  it('ignores invoices outside the window', async () => {
    const history = new InMemoryInvoiceHistoryRepo();
    for (let i = 0; i < 10; i++) {
      history.add({
        id: `inv_${i}`,
        accountRef: 'acc_1',
        amountCents: 100,
        createdAt: new Date('2026-05-10T08:00:00Z'),
      });
    }
    const out = await rule.evaluate(buildCtx(history));
    expect(out.matched).toBe(false);
  });
});
