import { DynamicModule, INestApplication, INestApplicationContext, Module } from '@nestjs/common';
import { NestFactory } from '@nestjs/core';
import {
  type Clock,
  DecisionThresholds,
  type FeatureSnapshotReader,
  type IDGen,
  type RegisteredRule,
  type ScoredAssessmentStore,
  ScoreRiskUseCase,
} from '@iris-gate/fraud';
import { loadRiskConfig, RISK_CONFIG, type RiskConfig } from '../../config/risk.config';
import { FraudModule } from '../fraud/fraud.module';
import {
  CLOCK,
  FEATURE_SNAPSHOT_READER,
  ID_GEN,
  REGISTERED_RULES,
  SCORED_ASSESSMENT_STORE,
} from '../fraud/tokens';
import { ProblemExceptionFilter } from './problem.filter';
import { RiskAssessmentController } from './risk-assessment.controller';
import { ServiceTokenGuard } from './service-token.guard';

/**
 * Wires `RiskConfig`, `ServiceTokenGuard`, and `ScoreRiskUseCase` inside `AppModule`
 * so startup fails immediately when `FRAUD_SERVICE_TOKEN` or `FraudRule` rows are
 * invalid. Does not register `RiskAssessmentController` on the public listener;
 * the controller is mounted only on `RiskInternalModule` (`ANTI_FRAUD_INTERNAL_PORT`).
 */
@Module({
  imports: [FraudModule],
  providers: [
    { provide: RISK_CONFIG, useFactory: () => loadRiskConfig() },
    ServiceTokenGuard,
    {
      provide: ScoreRiskUseCase,
      inject: [
        SCORED_ASSESSMENT_STORE,
        FEATURE_SNAPSHOT_READER,
        REGISTERED_RULES,
        CLOCK,
        ID_GEN,
        RISK_CONFIG,
      ],
      useFactory: (
        store: ScoredAssessmentStore,
        featureReader: FeatureSnapshotReader,
        rules: RegisteredRule[],
        clock: Clock,
        idGen: IDGen,
        config: RiskConfig,
      ): ScoreRiskUseCase =>
        new ScoreRiskUseCase(
          store,
          featureReader,
          rules,
          DecisionThresholds.defaults(),
          clock,
          idGen,
          config.testHooksEnabled,
        ),
    },
  ],
  exports: [RISK_CONFIG, ServiceTokenGuard, ScoreRiskUseCase],
})
export class RiskModule {}

export interface RiskInternalProviders {
  scoreRiskUseCase: ScoreRiskUseCase;
  serviceTokenGuard: ServiceTokenGuard;
  riskConfig: RiskConfig;
}

/**
 * Internal HTTP module mounted on the separate `ANTI_FRAUD_INTERNAL_PORT`
 * listener (never on the public `PORT` listener).
 */
@Module({})
export class RiskInternalModule {
  static register(providers: RiskInternalProviders): DynamicModule {
    return {
      module: RiskInternalModule,
      controllers: [RiskAssessmentController],
      providers: [
        ProblemExceptionFilter,
        { provide: ScoreRiskUseCase, useValue: providers.scoreRiskUseCase },
        { provide: ServiceTokenGuard, useValue: providers.serviceTokenGuard },
        { provide: RISK_CONFIG, useValue: providers.riskConfig },
      ],
    };
  }
}

/**
 * Builds the Nest HTTP application for the internal risk-scoring listener,
 * sharing the providers initialized by `parent`.
 */
export async function createInternalApp(
  parent: INestApplicationContext,
): Promise<INestApplication> {
  const internalApp = await NestFactory.create(
    RiskInternalModule.register({
      scoreRiskUseCase: parent.get(ScoreRiskUseCase),
      serviceTokenGuard: parent.get(ServiceTokenGuard),
      riskConfig: parent.get<RiskConfig>(RISK_CONFIG),
    }),
    { bufferLogs: true },
  );
  internalApp.useGlobalFilters(new ProblemExceptionFilter());
  return internalApp;
}
