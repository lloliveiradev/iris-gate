import { InMemoryInvoiceHistoryRepo } from '../fakes';
import { RuleContext } from './rule';
import { UnusualAmountSpec } from './unusual-amount.rule';

const buildCtx = (history: InMemoryInvoiceHistoryRepo, amount: number): RuleContext => ({
  account: { ref: 'acc_1', isSuspicious: false },
  amountCents: amount,
  paymentIntentId: 'pi_1',
  history,
  now: new Date('2026-05-10T12:00:00Z'),
});

describe('UnusualAmountSpec', () => {
  const rule = new UnusualAmountSpec({ historyCount: 5, thresholdPercent: 50 });

  it('does not match without history', async () => {
    const out = await rule.evaluate(buildCtx(new InMemoryInvoiceHistoryRepo(), 9_999_999));
    expect(out.matched).toBe(false);
  });

  it('matches when amount exceeds avg + threshold + avg', async () => {
    const history = new InMemoryInvoiceHistoryRepo();
    for (let i = 0; i < 5; i++) {
      history.add({
        id: `inv_${i}`,
        accountRef: 'acc_1',
        amountCents: 1000,
        createdAt: new Date('2026-05-10T11:00:00Z'),
      });
    }
    // avg = 1000; threshold = 1000 + 500 + 1000 = 2500. 5000 > 2500 → match.
    const out = await rule.evaluate(buildCtx(history, 5000));
    expect(out.matched).toBe(true);
    expect(out.baseScore).toBeGreaterThan(50);
    expect(out.description).toContain('exceeds');
  });

  it('does not match when amount is normal', async () => {
    const history = new InMemoryInvoiceHistoryRepo();
    for (let i = 0; i < 5; i++) {
      history.add({
        id: `inv_${i}`,
        accountRef: 'acc_1',
        amountCents: 1000,
        createdAt: new Date('2026-05-10T11:00:00Z'),
      });
    }
    const out = await rule.evaluate(buildCtx(history, 1500));
    expect(out.matched).toBe(false);
  });
});
