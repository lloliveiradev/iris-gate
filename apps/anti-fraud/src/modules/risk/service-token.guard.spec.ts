import type { ExecutionContext } from '@nestjs/common';
import { UnauthorizedProblemException } from './problem.filter';
import {
  MULTIBYTE_WRONG_TOKEN,
  TEST_SERVICE_TOKEN,
  testRiskConfig,
  WRONG_LONGER_TOKEN,
  WRONG_SAME_LENGTH_TOKEN,
  WRONG_SHORTER_TOKEN,
} from './risk.test-fixtures';
import { ServiceTokenGuard } from './service-token.guard';

function contextWithAuthorization(authorization?: string | string[]): ExecutionContext {
  return {
    switchToHttp: () => ({
      getRequest: () => ({
        headers: authorization === undefined ? {} : { authorization },
      }),
    }),
  } as unknown as ExecutionContext;
}

describe('ServiceTokenGuard', () => {
  it.each(['', '   '])('refuses to construct when serviceToken is blank (%p)', (serviceToken) => {
    expect(() => new ServiceTokenGuard(testRiskConfig({ serviceToken }))).toThrow(
      'FRAUD_SERVICE_TOKEN is not set',
    );
  });

  it('accepts a request carrying the configured Bearer token', () => {
    const guard = new ServiceTokenGuard(testRiskConfig());
    expect(guard.canActivate(contextWithAuthorization(`Bearer ${TEST_SERVICE_TOKEN}`))).toBe(true);
  });

  it.each<[string, string | string[] | undefined]>([
    ['missing header', undefined],
    ['empty header', ''],
    ['non-Bearer scheme', `Basic ${TEST_SERVICE_TOKEN}`],
    ['lowercase bearer prefix', `bearer ${TEST_SERVICE_TOKEN}`],
    ['Bearer with empty token', 'Bearer '],
    ['Bearer with trailing space', `Bearer ${TEST_SERVICE_TOKEN} `],
    ['array header', [`Bearer ${TEST_SERVICE_TOKEN}`]],
    ['wrong token of equal byte length', `Bearer ${WRONG_SAME_LENGTH_TOKEN}`],
    ['shorter wrong token', `Bearer ${WRONG_SHORTER_TOKEN}`],
    ['longer wrong token', `Bearer ${WRONG_LONGER_TOKEN}`],
    ['multi-byte UTF-8 wrong token', `Bearer ${MULTIBYTE_WRONG_TOKEN}`],
  ])('rejects %s with UnauthorizedProblemException and never leaks the token', (_case, header) => {
    const guard = new ServiceTokenGuard(testRiskConfig());
    let caught: unknown;
    try {
      guard.canActivate(contextWithAuthorization(header));
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(UnauthorizedProblemException);
    expect((caught as Error).message).not.toContain(TEST_SERVICE_TOKEN);
  });
});
