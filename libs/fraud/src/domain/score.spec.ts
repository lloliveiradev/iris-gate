import { InvalidScoreError } from './errors';
import { Score } from './score';

describe('Score', () => {
  it('accepts values in [0..100]', () => {
    expect(Score.of(0).value).toBe(0);
    expect(Score.of(50).value).toBe(50);
    expect(Score.of(100).value).toBe(100);
  });

  it.each([-1, 101, NaN, Infinity])('rejects invalid: %p', (v) => {
    expect(() => Score.of(v)).toThrow(InvalidScoreError);
  });

  it('add() saturates at 100', () => {
    expect(Score.of(80).add(Score.of(50)).value).toBe(100);
  });

  it('fromWeighted multiplies base by weight', () => {
    expect(Score.fromWeighted(80, 0.5).value).toBe(40);
    expect(Score.fromWeighted(100, 1).value).toBe(100);
    expect(Score.fromWeighted(0, 1).value).toBe(0);
  });

  it.each([
    [80, -0.1],
    [80, 1.5],
    [101, 0.5],
    [-1, 0.5],
  ])('fromWeighted rejects bad inputs: base=%p weight=%p', (base, w) => {
    expect(() => Score.fromWeighted(base, w)).toThrow(InvalidScoreError);
  });
});
