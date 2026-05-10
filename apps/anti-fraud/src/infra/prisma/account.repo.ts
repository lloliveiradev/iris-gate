import { Injectable } from '@nestjs/common';
import type { AccountRepo, AccountSnapshot } from '@iris-gate/fraud';
import { PrismaService } from './prisma.service';

@Injectable()
export class PrismaAccountRepo implements AccountRepo {
  constructor(private readonly prisma: PrismaService) {}

  async findOrCreate(ref: string): Promise<AccountSnapshot> {
    const account = await this.prisma.account.upsert({
      where: { ref },
      update: {},
      create: { ref },
    });
    return { ref: account.ref, isSuspicious: account.isSuspicious };
  }
}
