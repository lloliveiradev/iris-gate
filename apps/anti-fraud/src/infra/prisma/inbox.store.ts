import { Injectable } from '@nestjs/common';
import type { InboxStore } from '@iris-gate/fraud';
import { Prisma } from '@prisma/client';
import { PrismaService } from './prisma.service';

const UNIQUE_VIOLATION = 'P2002';

@Injectable()
export class PrismaInboxStore implements InboxStore {
  constructor(private readonly prisma: PrismaService) {}

  async reserve(messageId: string): Promise<{ accepted: boolean; assessmentId?: string }> {
    try {
      await this.prisma.inboxMessage.create({
        data: { messageId, topic: 'fraud.assessment.requested.v1' },
      });
      return { accepted: true };
    } catch (err) {
      if (err instanceof Prisma.PrismaClientKnownRequestError && err.code === UNIQUE_VIOLATION) {
        const existing = await this.prisma.inboxMessage.findUnique({ where: { messageId } });
        return existing?.assessmentId
          ? { accepted: false, assessmentId: existing.assessmentId }
          : { accepted: false };
      }
      throw err;
    }
  }

  async attachAssessment(messageId: string, assessmentId: string): Promise<void> {
    await this.prisma.inboxMessage.update({
      where: { messageId },
      data: { assessmentId, processedAt: new Date() },
    });
  }
}
