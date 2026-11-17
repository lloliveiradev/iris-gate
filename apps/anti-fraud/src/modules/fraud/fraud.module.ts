import { randomUUID } from 'node:crypto';
import { Module } from '@nestjs/common';
import {
  buildRulesFromConfig,
  type Clock,
  type FeatureSnapshotReader,
  type IDGen,
  type RegisteredRule,
  type RuleConfigStore,
} from '@iris-gate/fraud';
import { PrismaFeatureSnapshotReader } from '../../infra/prisma/feature-snapshot.reader';
import { PrismaOutboxStore } from '../../infra/prisma/outbox.store';
import { PrismaService } from '../../infra/prisma/prisma.service';
import { PrismaRuleConfigStore } from '../../infra/prisma/rule-config.store';
import { PrismaScoredAssessmentStore } from '../../infra/prisma/scored-assessment.store';
import { OUTBOX_STORE, OutboxRelayService } from '../../infra/kafka/outbox-relay.service';
import {
  CLOCK,
  FEATURE_SNAPSHOT_READER,
  ID_GEN,
  REGISTERED_RULES,
  RULE_CONFIG_STORE,
  SCORED_ASSESSMENT_STORE,
} from './tokens';

@Module({
  providers: [
    PrismaScoredAssessmentStore,
    PrismaOutboxStore,
    PrismaRuleConfigStore,
    { provide: SCORED_ASSESSMENT_STORE, useExisting: PrismaScoredAssessmentStore },
    { provide: OUTBOX_STORE, useExisting: PrismaOutboxStore },
    { provide: RULE_CONFIG_STORE, useExisting: PrismaRuleConfigStore },
    {
      provide: CLOCK,
      useValue: { now: () => new Date() } satisfies Clock,
    },
    {
      provide: ID_GEN,
      useValue: { newId: () => randomUUID() } satisfies IDGen,
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
  exports: [SCORED_ASSESSMENT_STORE, FEATURE_SNAPSHOT_READER, REGISTERED_RULES, CLOCK, ID_GEN],
})
export class FraudModule {}
