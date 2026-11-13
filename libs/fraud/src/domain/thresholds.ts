import { Decision } from './decision';
import { InvalidThresholdsError } from './errors';

export const DEFAULT_REVIEW_THRESHOLD = 30;
export const DEFAULT_REJECT_THRESHOLD = 70;

export class DecisionThresholds {
  private constructor(
    public readonly review: number,
    public readonly reject: number,
  ) {}

  static of(review: number, reject: number): DecisionThresholds {
    if (!Number.isFinite(review) || !Number.isFinite(reject)) {
      throw new InvalidThresholdsError('thresholds must be finite');
    }
    if (review < 0 || reject < 0 || review > 100 || reject > 100) {
      throw new InvalidThresholdsError('thresholds out of range [0..100]');
    }
    if (review > reject) {
      throw new InvalidThresholdsError(`review (${review}) must be <= reject (${reject})`);
    }
    return new DecisionThresholds(review, reject);
  }

  static defaults(): DecisionThresholds {
    return DecisionThresholds.of(DEFAULT_REVIEW_THRESHOLD, DEFAULT_REJECT_THRESHOLD);
  }

  classify(score: number): Decision {
    if (score >= this.reject) return Decision.REJECT;
    if (score >= this.review) return Decision.REVIEW;
    return Decision.APPROVE;
  }
}
