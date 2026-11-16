import { Module } from '@nestjs/common';
import {
  buildRulesFromConfig,
  type Clock,
  type FeatureSnapshotReader,
  type RegisteredRule,
  type RuleConfigStore,
} from '@iris-gate/fraud';
import { PrismaFeatureSnapshotReader } from '../../infra/prisma/feature-snapshot.reader';
import { PrismaInboxStore } from '../../infra/prisma/inbox.store';
import { PrismaOutboxStore } from '../../infra/prisma/outbox.store';
import { PrismaService } from '../../infra/prisma/prisma.service';
import { PrismaRiskAssessmentRepo } from '../../infra/prisma/risk-assessment.repo';
import { PrismaRuleConfigStore } from '../../infra/prisma/rule-config.store';
import { OUTBOX_STORE, OutboxRelayService } from '../../infra/kafka/outbox-relay.service';
import {
  FEATURE_SNAPSHOT_READER,
  INBOX_STORE,
  REGISTERED_RULES,
  RISK_ASSESSMENT_REPO,
  RULE_CONFIG_STORE,
} from './tokens';

const CLOCK = Symbol('CLOCK');

@Module({
  providers: [
    PrismaRiskAssessmentRepo,
    PrismaInboxStore,
    PrismaOutboxStore,
    PrismaRuleConfigStore,
    { provide: RISK_ASSESSMENT_REPO, useExisting: PrismaRiskAssessmentRepo },
    { provide: INBOX_STORE, useExisting: PrismaInboxStore },
    { provide: OUTBOX_STORE, useExisting: PrismaOutboxStore },
    { provide: RULE_CONFIG_STORE, useExisting: PrismaRuleConfigStore },
    {
      provide: CLOCK,
      useValue: { now: () => new Date() } satisfies Clock,
    },
    {
      provide: FEATURE_SNAPSHOT_READER,
      inject: [PrismaService, CLOCK],
      useFactory: (prisma: PrismaService, clock: Clock): FeatureSnapshotReader =>
        new PrismaFeatureSnapshotReader(prisma, clock),
    },
    {
      // Loaded once, at startup: a FraudRule row the registry rejects stops the
      // process instead of scoring with a partial rule set.
      provide: REGISTERED_RULES,
      inject: [RULE_CONFIG_STORE],
      useFactory: async (store: RuleConfigStore): Promise<RegisteredRule[]> =>
        buildRulesFromConfig(await store.load()),
    },
    OutboxRelayService,
  ],
})
export class FraudModule {}
