import { Module } from '@nestjs/common';
import {
  AssessRiskUseCase,
  buildRulesFromConfig,
  DecisionThresholds,
  type AccountRepo,
  type Clock,
  type IDGen,
  type InboxStore,
  type InvoiceHistoryRepo,
  type RiskAssessmentRepo,
  type RuleConfigStore,
} from '@iris-gate/fraud';
import { randomUUID } from 'node:crypto';
import { PrismaAccountRepo } from '../../infra/prisma/account.repo';
import { PrismaInboxStore } from '../../infra/prisma/inbox.store';
import { PrismaInvoiceHistoryRepo } from '../../infra/prisma/invoice-history.repo';
import { PrismaOutboxStore } from '../../infra/prisma/outbox.store';
import { PrismaRiskAssessmentRepo } from '../../infra/prisma/risk-assessment.repo';
import { PrismaRuleConfigStore } from '../../infra/prisma/rule-config.store';
import { AssessmentConsumer } from '../../infra/kafka/assessment.consumer';
import { OUTBOX_STORE, OutboxRelayService } from '../../infra/kafka/outbox-relay.service';
import {
  ACCOUNT_REPO,
  INBOX_STORE,
  INVOICE_HISTORY_REPO,
  RISK_ASSESSMENT_REPO,
  RULE_CONFIG_STORE,
} from './tokens';

const CLOCK = Symbol('CLOCK');
const ID_GEN = Symbol('ID_GEN');

@Module({
  providers: [
    PrismaAccountRepo,
    PrismaInvoiceHistoryRepo,
    PrismaRiskAssessmentRepo,
    PrismaInboxStore,
    PrismaOutboxStore,
    PrismaRuleConfigStore,
    { provide: ACCOUNT_REPO, useExisting: PrismaAccountRepo },
    { provide: INVOICE_HISTORY_REPO, useExisting: PrismaInvoiceHistoryRepo },
    { provide: RISK_ASSESSMENT_REPO, useExisting: PrismaRiskAssessmentRepo },
    { provide: INBOX_STORE, useExisting: PrismaInboxStore },
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
      provide: AssessRiskUseCase,
      inject: [
        INBOX_STORE,
        ACCOUNT_REPO,
        INVOICE_HISTORY_REPO,
        RISK_ASSESSMENT_REPO,
        RULE_CONFIG_STORE,
        CLOCK,
        ID_GEN,
      ],
      useFactory: async (
        inbox: InboxStore,
        accounts: AccountRepo,
        history: InvoiceHistoryRepo,
        assessments: RiskAssessmentRepo,
        rules: RuleConfigStore,
        clock: Clock,
        idGen: IDGen,
      ) => {
        const configs = await rules.load();
        const built = buildRulesFromConfig(configs);
        return new AssessRiskUseCase(
          inbox,
          accounts,
          history,
          assessments,
          built,
          DecisionThresholds.defaults(),
          clock,
          idGen,
        );
      },
    },
    AssessmentConsumer,
    OutboxRelayService,
  ],
  exports: [AssessRiskUseCase],
})
export class FraudModule {}
