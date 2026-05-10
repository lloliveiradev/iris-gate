import { Decision } from './decision';
import { InvalidThresholdsError } from './errors';
import { DecisionThresholds } from './thresholds';

describe('DecisionThresholds', () => {
  const t = DecisionThresholds.defaults();

  it.each([
    [0, Decision.APPROVE],
    [29.999, Decision.APPROVE],
    [30, Decision.REVIEW],
    [69.999, Decision.REVIEW],
    [70, Decision.REJECT],
    [100, Decision.REJECT],
  ])('classify(%p) = %s', (score, expected) => {
    expect(t.classify(score)).toBe(expected);
  });

  it('rejects review > reject', () => {
    expect(() => DecisionThresholds.of(80, 70)).toThrow(InvalidThresholdsError);
  });

  it.each([
    [-1, 70],
    [30, 101],
    [Number.NaN, 70],
  ])('rejects out-of-range: review=%p reject=%p', (r, j) => {
    expect(() => DecisionThresholds.of(r, j)).toThrow(InvalidThresholdsError);
  });
});
