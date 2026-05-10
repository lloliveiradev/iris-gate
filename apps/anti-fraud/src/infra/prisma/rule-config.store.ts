import { Injectable, Logger } from '@nestjs/common';
import { type RuleConfig, type RuleConfigStore, DEFAULT_RULES } from '@iris-gate/fraud';
import { PrismaService } from './prisma.service';

@Injectable()
export class PrismaRuleConfigStore implements RuleConfigStore {
  private readonly logger = new Logger(PrismaRuleConfigStore.name);

  constructor(private readonly prisma: PrismaService) {}

  async load(): Promise<RuleConfig[]> {
    const rows = await this.prisma.fraudRule.findMany();
    if (rows.length === 0) {
      this.logger.warn('no FraudRule rows; using DEFAULT_RULES');
      return DEFAULT_RULES;
    }
    return rows.map((r) => ({
      code: r.code,
      weight: r.weight,
      enabled: r.enabled,
      params: r.params as unknown as Record<string, number>,
    }));
  }
}
