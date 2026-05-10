export interface RawAssessmentRequestPayload {
  event_id?: unknown;
  occurred_at?: unknown;
  assessment_id?: unknown;
  payment_intent_id?: unknown;
  merchant_id?: unknown;
  account_ref?: unknown;
  amount_cents?: unknown;
  currency?: unknown;
}

export interface ParsedAssessmentRequest {
  eventId: string;
  occurredAt: Date;
  assessmentId: string;
  paymentIntentId: string;
  merchantId: string;
  accountRef: string;
  amountCents: number;
  currency: string;
}

export class PayloadValidationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'PayloadValidationError';
  }
}

const requireString = (raw: unknown, field: string): string => {
  if (typeof raw !== 'string' || raw.length === 0) {
    throw new PayloadValidationError(`field '${field}' must be a non-empty string`);
  }
  return raw;
};

const requireInt = (raw: unknown, field: string): number => {
  if (typeof raw !== 'number' || !Number.isInteger(raw) || raw <= 0) {
    throw new PayloadValidationError(`field '${field}' must be a positive integer`);
  }
  return raw;
};

const requireDate = (raw: unknown, field: string): Date => {
  const s = requireString(raw, field);
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) {
    throw new PayloadValidationError(`field '${field}' must be a valid ISO-8601 date`);
  }
  return d;
};

export function parseAssessmentRequest(raw: RawAssessmentRequestPayload): ParsedAssessmentRequest {
  const currency = requireString(raw.currency, 'currency');
  if (!/^[A-Z]{3}$/.test(currency)) {
    throw new PayloadValidationError(`field 'currency' must match /^[A-Z]{3}$/`);
  }
  return {
    eventId: requireString(raw.event_id, 'event_id'),
    occurredAt: requireDate(raw.occurred_at, 'occurred_at'),
    assessmentId: requireString(raw.assessment_id, 'assessment_id'),
    paymentIntentId: requireString(raw.payment_intent_id, 'payment_intent_id'),
    merchantId: requireString(raw.merchant_id, 'merchant_id'),
    accountRef: requireString(raw.account_ref, 'account_ref'),
    amountCents: requireInt(raw.amount_cents, 'amount_cents'),
    currency,
  };
}
