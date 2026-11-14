import { Decision, mostSevere } from './decision';

describe('mostSevere', () => {
  it.each([
    [Decision.APPROVE, Decision.APPROVE, Decision.APPROVE],
    [Decision.APPROVE, Decision.REVIEW, Decision.REVIEW],
    [Decision.REVIEW, Decision.APPROVE, Decision.REVIEW],
    [Decision.REVIEW, Decision.REJECT, Decision.REJECT],
    [Decision.REJECT, Decision.REVIEW, Decision.REJECT],
    [Decision.APPROVE, Decision.REJECT, Decision.REJECT],
    [Decision.REJECT, Decision.REJECT, Decision.REJECT],
  ])('mostSevere(%s, %s) = %s', (a, b, expected) => {
    expect(mostSevere(a, b)).toBe(expected);
  });
});
