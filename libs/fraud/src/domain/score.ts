import { InvalidScoreError } from './errors';

const MIN = 0;
const MAX = 100;

export class Score {
  private constructor(public readonly value: number) {}

  static of(value: number): Score {
    if (!Number.isFinite(value)) {
      throw new InvalidScoreError(`score must be finite, got ${value}`);
    }
    if (value < MIN || value > MAX) {
      throw new InvalidScoreError(`score out of range [${MIN}..${MAX}]: ${value}`);
    }
    return new Score(value);
  }

  static zero(): Score {
    return new Score(0);
  }

  add(other: Score): Score {
    return Score.of(Math.min(MAX, this.value + other.value));
  }

  static fromWeighted(baseScore: number, weight: number): Score {
    if (!Number.isFinite(baseScore) || !Number.isFinite(weight)) {
      throw new InvalidScoreError('weight and baseScore must be finite');
    }
    if (weight < 0 || weight > 1) {
      throw new InvalidScoreError(`weight out of range [0..1]: ${weight}`);
    }
    if (baseScore < 0 || baseScore > MAX) {
      throw new InvalidScoreError(`baseScore out of range [0..${MAX}]: ${baseScore}`);
    }
    return new Score(baseScore * weight);
  }
}
