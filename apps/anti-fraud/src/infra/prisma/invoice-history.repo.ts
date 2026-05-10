import { Injectable } from '@nestjs/common';
import type { InvoiceHistoryRepo, InvoiceRecord } from '@iris-gate/fraud';
import { PrismaService } from './prisma.service';

@Injectable()
export class PrismaInvoiceHistoryRepo implements InvoiceHistoryRepo {
  constructor(private readonly prisma: PrismaService) {}

  async recentByAccount(accountRef: string, since: Date): Promise<InvoiceRecord[]> {
    const rows = await this.prisma.invoiceAttempt.findMany({
      where: { accountRef, occurredAt: { gt: since } },
      orderBy: { occurredAt: 'desc' },
    });
    return rows.map((r) => ({
      id: r.paymentIntentId,
      accountRef: r.accountRef,
      amountCents: r.amountCents,
      createdAt: r.occurredAt,
    }));
  }

  async lastNByAccount(accountRef: string, limit: number): Promise<InvoiceRecord[]> {
    const rows = await this.prisma.invoiceAttempt.findMany({
      where: { accountRef },
      orderBy: { occurredAt: 'desc' },
      take: limit,
    });
    return rows.map((r) => ({
      id: r.paymentIntentId,
      accountRef: r.accountRef,
      amountCents: r.amountCents,
      createdAt: r.occurredAt,
    }));
  }

  async recordAttempt(input: {
    paymentIntentId: string;
    accountRef: string;
    amountCents: number;
    occurredAt: Date;
  }): Promise<void> {
    await this.prisma.invoiceAttempt.upsert({
      where: { paymentIntentId: input.paymentIntentId },
      update: {},
      create: input,
    });
  }
}
