import { parseAssessmentRequest, PayloadValidationError } from './payload';

const valid = {
  event_id: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
  occurred_at: '2026-05-10T12:00:00Z',
  assessment_id: 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
  payment_intent_id: 'cccccccc-cccc-cccc-cccc-cccccccccccc',
  merchant_id: 'dddddddd-dddd-dddd-dddd-dddddddddddd',
  account_ref: 'acc_42',
  amount_cents: 1500,
  currency: 'BRL',
};

describe('parseAssessmentRequest', () => {
  it('parses a valid payload', () => {
    const out = parseAssessmentRequest(valid);
    expect(out.eventId).toBe(valid.event_id);
    expect(out.amountCents).toBe(1500);
    expect(out.occurredAt).toBeInstanceOf(Date);
  });

  it.each([
    ['event_id', { event_id: '' }],
    ['account_ref missing', { account_ref: undefined }],
    ['amount_cents fractional', { amount_cents: 1.5 }],
    ['amount_cents zero', { amount_cents: 0 }],
    ['currency lowercase', { currency: 'brl' }],
    ['occurred_at non-iso', { occurred_at: 'not-a-date' }],
  ])('rejects bad input: %s', (_label, override) => {
    expect(() => parseAssessmentRequest({ ...valid, ...override })).toThrow(
      PayloadValidationError,
    );
  });
});
