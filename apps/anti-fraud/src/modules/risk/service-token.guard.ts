import type { IncomingHttpHeaders } from 'node:http';
import { timingSafeEqual } from 'node:crypto';
import { CanActivate, ExecutionContext, Inject, Injectable } from '@nestjs/common';
import { RISK_CONFIG, type RiskConfig } from '../../config/risk.config';
import { UnauthorizedProblemException } from './problem.filter';

interface AuthenticatedHttpRequest {
  headers: IncomingHttpHeaders;
}

const BEARER_PREFIX = 'Bearer ';

/**
 * Authenticates requests to the internal risk-scoring listener against
 * `FRAUD_SERVICE_TOKEN` using `crypto.timingSafeEqual` on equal-length buffers.
 * Never logs the expected or provided token.
 */
@Injectable()
export class ServiceTokenGuard implements CanActivate {
  private readonly expectedToken: Buffer;

  constructor(@Inject(RISK_CONFIG) config: RiskConfig) {
    const token = config.serviceToken.trim();
    if (token.length === 0) {
      throw new Error('FRAUD_SERVICE_TOKEN is not set');
    }
    this.expectedToken = Buffer.from(token, 'utf8');
  }

  canActivate(context: ExecutionContext): boolean {
    const request = context.switchToHttp().getRequest<AuthenticatedHttpRequest>();
    const candidate = extractBearerToken(request.headers.authorization);
    if (!this.matches(candidate)) {
      throw new UnauthorizedProblemException('missing or invalid service token');
    }
    return true;
  }

  private matches(candidate: string | null): boolean {
    const provided = Buffer.from(candidate ?? '', 'utf8');
    const sameLength = provided.byteLength === this.expectedToken.byteLength;
    const comparable = sameLength ? provided : this.expectedToken;
    return timingSafeEqual(comparable, this.expectedToken) && sameLength;
  }
}

function extractBearerToken(header: string | string[] | undefined): string | null {
  if (typeof header !== 'string' || !header.startsWith(BEARER_PREFIX)) {
    return null;
  }
  const token = header.slice(BEARER_PREFIX.length);
  return token.length > 0 && !/\s/.test(token) ? token : null;
}
