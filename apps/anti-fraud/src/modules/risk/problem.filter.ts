import {
  ArgumentsHost,
  BadRequestException,
  Catch,
  ExceptionFilter,
  HttpException,
  HttpStatus,
  Logger,
} from '@nestjs/common';
import { AssessmentConflictError, InvalidAssessmentInputError } from '@iris-gate/fraud';
import { InvalidRiskRequestError, type ProblemDto } from './risk-assessment.dto';

interface HttpProblemResponse {
  status(code: number): {
    header(
      name: string,
      value: string,
    ): {
      json(body: ProblemDto): void;
    };
  };
}

const PROBLEM_CONTENT_TYPE = 'application/problem+json';
const BLANK_PROBLEM_TYPE = 'about:blank';

/** Missing or invalid Bearer service token on the internal risk endpoint. */
export class UnauthorizedProblemException extends Error {
  constructor(message = 'missing or invalid service token') {
    super(message);
    this.name = new.target.name;
  }
}

/**
 * Formats every error raised on the internal risk listener as RFC 7807
 * `application/problem+json` (`FraudInternal.Problem`).
 */
@Catch()
export class ProblemExceptionFilter implements ExceptionFilter {
  private readonly logger = new Logger(ProblemExceptionFilter.name);

  catch(exception: unknown, host: ArgumentsHost): void {
    const response = host.switchToHttp().getResponse<HttpProblemResponse>();
    const problem = this.toProblem(exception);
    response.status(problem.status).header('Content-Type', PROBLEM_CONTENT_TYPE).json(problem);
  }

  private toProblem(exception: unknown): ProblemDto {
    if (exception instanceof UnauthorizedProblemException) {
      return buildProblem(HttpStatus.UNAUTHORIZED, 'unauthorized', exception.message);
    }
    if (
      exception instanceof InvalidRiskRequestError ||
      exception instanceof InvalidAssessmentInputError
    ) {
      return buildProblem(HttpStatus.BAD_REQUEST, 'validation', exception.message);
    }
    if (exception instanceof BadRequestException) {
      return buildProblem(HttpStatus.BAD_REQUEST, 'validation', exception.message);
    }
    if (exception instanceof AssessmentConflictError) {
      return buildProblem(HttpStatus.CONFLICT, 'assessment_conflict', exception.message);
    }
    if (exception instanceof HttpException) {
      const status = exception.getStatus();
      return buildProblem(status, 'http_error', exception.message);
    }

    const message = exception instanceof Error ? exception.message : 'unexpected error';
    this.logger.error(`internal risk assessment failed: ${message}`);
    return buildProblem(HttpStatus.INTERNAL_SERVER_ERROR, 'internal', 'internal error');
  }
}

function buildProblem(status: number, title: string, detail: string): ProblemDto {
  return {
    type: BLANK_PROBLEM_TYPE,
    title,
    status,
    detail,
  };
}
