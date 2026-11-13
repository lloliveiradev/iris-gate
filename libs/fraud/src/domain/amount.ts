import { InvalidAssessmentInputError } from './errors';

/**
 * Guards an amount in integer minor units as it enters the domain. Amounts stay
 * JavaScript numbers, so they must be safe integers; anything above 2^31 - 1 is
 * valid as long as it is below `Number.MAX_SAFE_INTEGER`.
 */
export function assertAmountCents(amountCents: number): void {
  if (!Number.isSafeInteger(amountCents) || amountCents <= 0) {
    throw new InvalidAssessmentInputError(`invalid amountCents: ${amountCents}`);
  }
}
