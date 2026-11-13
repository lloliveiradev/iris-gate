import { Decision } from '../domain/decision';
import { InvalidAssessmentInputError, InvalidFeatureSnapshotError } from '../domain/errors';
import { DecisionThresholds } from '../domain/thresholds';
import { evaluateRules } from './evaluate-rules';
import { FeatureSnapshot } from './feature-snapshot';
import {
  ABOVE_INT32_AMOUNT_CENTS,
  FIXTURE_NOW,
  firstTimeCardFeatures,
  NORMAL_AMOUNT_CENTS,
  UNSAFE_AMOUNT_CENTS,
} from './fixtures';
import { buildRulesFromConfig, DEFAULT_RULES } from './rule-registry';

const DEFAULT_MAX_ATTEMPTS = 5;
const DEFAULT_MAX_DECLINES = 3;

const rules = buildRulesFromConfig(DEFAULT_RULES);
const thresholds = DecisionThresholds.defaults();

const evaluate = (features: FeatureSnapshot, amountCents = NORMAL_AMOUNT_CENTS) =>
  evaluateRules(rules, { features, amountCents, now: FIXTURE_NOW }, thresholds);

describe('evaluateRules with DEFAULT_RULES (phase 10 AC1)', () => {
  it('approves a first-time card with a normal amount', () => {
    expect(evaluate(firstTimeCardFeatures())).toEqual({
      decision: Decision.APPROVE,
      score: 0,
      reasons: [],
    });
  });

  it('reviews a card one attempt over the velocity limit', () => {
    const out = evaluate(firstTimeCardFeatures({ attempts24h: DEFAULT_MAX_ATTEMPTS + 1 }));
    expect(out.decision).toBe(Decision.REVIEW);
    expect(out.score).toBe(35);
    expect(out.reasons.map((r) => r.code)).toEqual(['VELOCITY']);
  });

  it('approves a card at the velocity limit', () => {
    const out = evaluate(firstTimeCardFeatures({ attempts24h: DEFAULT_MAX_ATTEMPTS }));
    expect(out.decision).toBe(Decision.APPROVE);
  });

  it('rejects a blocked card', () => {
    const out = evaluate(firstTimeCardFeatures({ blocked: true }));
    expect(out.decision).toBe(Decision.REJECT);
    expect(out.score).toBe(100);
    expect(out.reasons).toEqual([
      { code: 'BLOCKED_CARD', weight: 1, score: 100, description: 'card is blocked' },
    ]);
  });

  it('reviews a card one decline over the decline-burst limit', () => {
    const out = evaluate(firstTimeCardFeatures({ declines24h: DEFAULT_MAX_DECLINES + 1 }));
    expect(out.decision).toBe(Decision.REVIEW);
    expect(out.score).toBeCloseTo(49);
    expect(out.reasons.map((r) => r.code)).toEqual(['DECLINE_BURST']);
  });

  it('adds matching rules and caps the total at 100', () => {
    const out = evaluate(
      firstTimeCardFeatures({
        blocked: true,
        attempts24h: DEFAULT_MAX_ATTEMPTS + 10,
        declines24h: DEFAULT_MAX_DECLINES + 10,
      }),
    );
    expect(out.score).toBe(100);
    expect(out.reasons.map((r) => r.code)).toEqual(['BLOCKED_CARD', 'VELOCITY', 'DECLINE_BURST']);
  });
});

describe('evaluateRules with large amounts (phase 10 AC3)', () => {
  it.each([ABOVE_INT32_AMOUNT_CENTS, 10 * ABOVE_INT32_AMOUNT_CENTS, Number.MAX_SAFE_INTEGER])(
    'approves %p cents on a first-time card',
    (amountCents) => {
      expect(evaluate(firstTimeCardFeatures(), amountCents).decision).toBe(Decision.APPROVE);
    },
  );

  it('approves a large amount in line with a large captured average', () => {
    const features = firstTimeCardFeatures({ avgCapturedCents: ABOVE_INT32_AMOUNT_CENTS });
    expect(evaluate(features, ABOVE_INT32_AMOUNT_CENTS + 1).decision).toBe(Decision.APPROVE);
  });

  it('reviews a large amount far above a large captured average', () => {
    const features = firstTimeCardFeatures({ avgCapturedCents: ABOVE_INT32_AMOUNT_CENTS });
    const out = evaluate(features, 4 * ABOVE_INT32_AMOUNT_CENTS);
    expect(out.decision).toBe(Decision.REVIEW);
    expect(out.reasons.map((r) => r.code)).toEqual(['UNUSUAL_AMOUNT']);
  });
});

describe('evaluateRules fails closed on malformed input', () => {
  it.each([0, -1, 1.5, Number.NaN, UNSAFE_AMOUNT_CENTS])('throws on amount %p', (amountCents) => {
    expect(() => evaluate(firstTimeCardFeatures(), amountCents)).toThrow(
      InvalidAssessmentInputError,
    );
  });

  it('throws on a snapshot that was not validated', () => {
    const malformed = { ...firstTimeCardFeatures(), attempts24h: Number.NaN };
    expect(() => evaluate(malformed)).toThrow(InvalidFeatureSnapshotError);
  });
});
