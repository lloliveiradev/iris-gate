import {
  Body,
  Controller,
  HttpCode,
  HttpStatus,
  Post,
  UseFilters,
  UseGuards,
} from '@nestjs/common';
import { ScoreRiskUseCase } from '@iris-gate/fraud';
import { ProblemExceptionFilter } from './problem.filter';
import {
  parseRiskAssessmentRequest,
  type RiskAssessmentResponseDto,
  toRiskAssessmentResponse,
} from './risk-assessment.dto';
import { ServiceTokenGuard } from './service-token.guard';

export const RISK_ASSESSMENTS_PATH = 'internal/v1/risk-assessments';

/**
 * Synchronous risk-scoring endpoint (`POST /internal/v1/risk-assessments`),
 * mounted only on the internal listener (`ANTI_FRAUD_INTERNAL_PORT`).
 */
@Controller()
@UseFilters(ProblemExceptionFilter)
@UseGuards(ServiceTokenGuard)
export class RiskAssessmentController {
  constructor(private readonly scoreRisk: ScoreRiskUseCase) {}

  @Post(RISK_ASSESSMENTS_PATH)
  @HttpCode(HttpStatus.OK)
  async create(@Body() body: unknown): Promise<RiskAssessmentResponseDto> {
    const input = parseRiskAssessmentRequest(body);
    const output = await this.scoreRisk.execute(input);
    return toRiskAssessmentResponse(output);
  }
}
