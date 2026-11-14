import { Decision } from '../domain/decision';
import {
  DuplicateAssessmentError,
  IncompleteAssessmentError,
  InvalidAssessmentInputError,
  InvalidFeatureSnapshotError,
} from '../domain/errors';
import { AssessmentStatus } from '../domain/risk-assessment';
import { TestScenario } from '../domain/test-scenario';
import { DecisionThresholds } from '../domain/thresholds';
import { InMemoryFeatureSnapshotReader, SequentialIdGen } from './fakes';
import { FeatureSnapshot, FeatureSnapshotReader } from './feature-snapshot';
import { ABOVE_INT32_AMOUNT_CENTS, firstTimeCardFeatures } from './fixtures';
import { Clock } from './ports';
import { buildRulesFromConfig, DEFAULT_RULES } from './rule-registry';
import {
  completedAssessment,
  FAILING_READER_ERROR_MESSAGE,
  FailingFeatureSnapshotReader,
  FAKE_CARD_BRAND,
  FAKE_CARD_FINGERPRINT,
  FAKE_CURRENCY,
  FAKE_CUSTOMER_REF,
  FAKE_MERCHANT_ID,
  FAKE_OTHER_PAYMENT_INTENT_ID,
  FAKE_PAYMENT_INTENT_ID,
  FAKE_REQUESTED_AT,
  FAKE_STORED_ASSESSMENT_ID,
  InMemoryScoredAssessmentStore,
  pendingAssessment,
  scoreRiskInput,
} from './score-risk.fakes';
import { ASSESSMENT_COMPLETED_TOPIC, ScoreRiskUseCase } from './score-risk.use-case';

const DEFAULT_MAX_ATTEMPTS = 5;
const SCORING_STEP_MS = 7;
const SAVE_FAILURE_MESSAGE = 'database unavailable';

/** Clock that moves forward on every read, so requestedAt and completedAt differ. */
class SteppingClock implements Clock {
  private current = FAKE_REQUESTED_AT.getTime();

  now(): Date {
    const at = new Date(this.current);
    this.current += SCORING_STEP_MS;
    return at;
  }
}

interface Setup {
  reader?: FeatureSnapshotReader;
  testHooksEnabled?: boolean;
}

function setup({ reader, testHooksEnabled }: Setup = {}) {
  const store = new InMemoryScoredAssessmentStore();
  const features = new InMemoryFeatureSnapshotReader();
  const useCase = new ScoreRiskUseCase(
    store,
    reader ?? features,
    buildRulesFromConfig(DEFAULT_RULES),
    DecisionThresholds.defaults(),
    new SteppingClock(),
    new SequentialIdGen(),
    testHooksEnabled,
  );
  return { store, features, useCase };
}

describe('ScoreRiskUseCase', () => {
  describe('rules on a new payment intent (phase 10 AC1)', () => {
    it('approves a first-time card with a normal amount', async () => {
      const { useCase } = setup();
      await expect(useCase.execute(scoreRiskInput())).resolves.toEqual({
        assessmentId: 'gen_001',
        decision: Decision.APPROVE,
        score: 0,
        reasons: [],
        deduplicated: false,
      });
    });

    it('gives at least REVIEW once the attempts go over the velocity limit', async () => {
      const { features, useCase } = setup();
      features.set(firstTimeCardFeatures({ attempts24h: DEFAULT_MAX_ATTEMPTS }));
      const out = await useCase.execute(scoreRiskInput());
      expect([Decision.REVIEW, Decision.REJECT]).toContain(out.decision);
      expect(out.reasons.map((r) => r.code)).toEqual(['VELOCITY']);
    });

    it('rejects a blocked card', async () => {
      const { features, useCase } = setup();
      features.set(firstTimeCardFeatures({ blocked: true }));
      const out = await useCase.execute(scoreRiskInput());
      expect(out.decision).toBe(Decision.REJECT);
      expect(out.reasons.map((r) => r.code)).toEqual(['BLOCKED_CARD']);
    });

    it('scores an amount above 2^31 cents and records it exactly (AC3)', async () => {
      const { store, useCase } = setup();
      const out = await useCase.execute(scoreRiskInput({ amountCents: ABOVE_INT32_AMOUNT_CENTS }));
      expect(out.decision).toBe(Decision.APPROVE);
      expect(store.saved[0]?.attempt.amountCents).toBe(ABOVE_INT32_AMOUNT_CENTS);
      expect(store.saved[0]?.assessment.amountCents).toBe(ABOVE_INT32_AMOUNT_CENTS);
    });
  });

  describe('what a new payment intent writes', () => {
    it('loads one snapshot for the card and customer', async () => {
      const { features, useCase } = setup();
      await useCase.execute(scoreRiskInput());
      expect(features.queries).toEqual([
        { cardFingerprint: FAKE_CARD_FINGERPRINT, customerRef: FAKE_CUSTOMER_REF },
      ]);
    });

    it('saves the completed assessment, the snapshot, the attempt and the outbox event together', async () => {
      const { features, store, useCase } = setup();
      const seen = firstTimeCardFeatures({ attempts24h: 2, avgCapturedCents: 900 });
      features.set(seen);

      await useCase.execute(scoreRiskInput());

      expect(store.saved).toHaveLength(1);
      const [scored] = store.saved;
      expect(scored?.assessment.toSnapshot()).toEqual({
        id: 'gen_001',
        paymentIntentId: FAKE_PAYMENT_INTENT_ID,
        merchantId: FAKE_MERCHANT_ID,
        accountRef: FAKE_CARD_FINGERPRINT,
        amountCents: scoreRiskInput().amountCents,
        currency: FAKE_CURRENCY,
        status: AssessmentStatus.COMPLETED,
        decision: Decision.APPROVE,
        score: 0,
        reasons: [],
        requestedAt: FAKE_REQUESTED_AT,
        completedAt: new Date(FAKE_REQUESTED_AT.getTime() + SCORING_STEP_MS),
      });
      expect(scored?.features).toEqual(seen);
      expect(Object.isFrozen(scored?.features)).toBe(true);
      expect(scored?.attempt).toEqual({
        paymentIntentId: FAKE_PAYMENT_INTENT_ID,
        merchantId: FAKE_MERCHANT_ID,
        cardFingerprint: FAKE_CARD_FINGERPRINT,
        cardBrand: FAKE_CARD_BRAND,
        customerRef: FAKE_CUSTOMER_REF,
        amountCents: scoreRiskInput().amountCents,
        currency: FAKE_CURRENCY,
        occurredAt: FAKE_REQUESTED_AT,
      });
    });

    it('writes fraud.assessment.completed.v1 with one event id for the record and the payload', async () => {
      const { features, store, useCase } = setup();
      features.set(firstTimeCardFeatures({ blocked: true }));

      await useCase.execute(scoreRiskInput());

      const completedAt = new Date(FAKE_REQUESTED_AT.getTime() + SCORING_STEP_MS);
      expect(store.saved[0]?.outboxEvent).toEqual({
        eventId: 'gen_002',
        eventType: ASSESSMENT_COMPLETED_TOPIC,
        topic: ASSESSMENT_COMPLETED_TOPIC,
        occurredAt: completedAt,
        payload: {
          event_id: 'gen_002',
          occurred_at: completedAt.toISOString(),
          assessment_id: 'gen_001',
          payment_intent_id: FAKE_PAYMENT_INTENT_ID,
          merchant_id: FAKE_MERCHANT_ID,
          decision: Decision.REJECT,
          score: 100,
          reasons: [
            { code: 'BLOCKED_CARD', weight: 1, score: 100, description: 'card is blocked' },
          ],
        },
      });
    });

    it('records a missing customer as null and queries by card only', async () => {
      const { features, store, useCase } = setup();
      await useCase.execute(scoreRiskInput({ customerRef: undefined }));
      expect(store.saved[0]?.attempt.customerRef).toBeNull();
      expect(features.queries).toEqual([
        { cardFingerprint: FAKE_CARD_FINGERPRINT, customerRef: undefined },
      ]);
    });
  });

  describe('idempotency by payment_intent_id (phase 10 AC2)', () => {
    it('returns the stored assessment on a repeat without rescoring or writing again', async () => {
      const { features, store, useCase } = setup();
      const first = await useCase.execute(scoreRiskInput());

      // The card is blocked now; a rescore would REJECT.
      features.set(firstTimeCardFeatures({ blocked: true }));
      const repeat = await useCase.execute(scoreRiskInput());

      expect(repeat).toEqual({ ...first, deduplicated: true });
      expect(store.saved).toHaveLength(1);
      expect(features.queries).toHaveLength(1);
    });

    it('scores a different payment intent for the same card', async () => {
      const { store, useCase } = setup();
      await useCase.execute(scoreRiskInput());
      const other = await useCase.execute(
        scoreRiskInput({ paymentIntentId: FAKE_OTHER_PAYMENT_INTENT_ID }),
      );
      expect(other.deduplicated).toBe(false);
      expect(store.saved).toHaveLength(2);
    });

    it('answers from an assessment stored earlier without loading features', async () => {
      const { features, store, useCase } = setup();
      const reasons = [
        { code: 'BLOCKED_CARD' as const, weight: 1, score: 100, description: 'card is blocked' },
      ];
      store.seed(completedAssessment({ decision: Decision.REJECT, score: 100, reasons }));

      await expect(useCase.execute(scoreRiskInput())).resolves.toEqual({
        assessmentId: FAKE_STORED_ASSESSMENT_ID,
        decision: Decision.REJECT,
        score: 100,
        reasons,
        deduplicated: true,
      });
      expect(features.queries).toEqual([]);
      expect(store.saved).toEqual([]);
    });

    it('fails instead of approving when the stored assessment has no decision', async () => {
      const { store, useCase } = setup();
      store.seed(pendingAssessment());
      await expect(useCase.execute(scoreRiskInput())).rejects.toThrow(IncompleteAssessmentError);
      expect(store.saved).toEqual([]);
    });

    it('returns the winner when a concurrent request stored the assessment first', async () => {
      const { store, useCase } = setup();
      store.loseNextSaveTo(
        completedAssessment({ decision: Decision.REVIEW, score: 35, reasons: [] }),
      );

      await expect(useCase.execute(scoreRiskInput())).resolves.toEqual({
        assessmentId: FAKE_STORED_ASSESSMENT_ID,
        decision: Decision.REVIEW,
        score: 35,
        reasons: [],
        deduplicated: true,
      });
      expect(store.saved).toEqual([]);
    });

    it('rethrows the duplicate error when the winner cannot be reloaded', async () => {
      const { store, useCase } = setup();
      store.failNextSave(new DuplicateAssessmentError(FAKE_PAYMENT_INTENT_ID));
      await expect(useCase.execute(scoreRiskInput())).rejects.toThrow(DuplicateAssessmentError);
    });
  });

  describe('fails closed', () => {
    it('propagates a feature snapshot failure and writes nothing', async () => {
      const { store, useCase } = setup({ reader: new FailingFeatureSnapshotReader() });
      await expect(useCase.execute(scoreRiskInput())).rejects.toThrow(FAILING_READER_ERROR_MESSAGE);
      expect(store.saved).toEqual([]);
    });

    it('propagates a malformed snapshot instead of scoring it', async () => {
      const malformed: FeatureSnapshot = { ...firstTimeCardFeatures(), attempts24h: Number.NaN };
      const { store, useCase } = setup({ reader: { load: async () => malformed } });
      await expect(useCase.execute(scoreRiskInput())).rejects.toThrow(InvalidFeatureSnapshotError);
      expect(store.saved).toEqual([]);
    });

    it('propagates a store failure other than a duplicate', async () => {
      const { store, useCase } = setup();
      store.failNextSave(new Error(SAVE_FAILURE_MESSAGE));
      await expect(useCase.execute(scoreRiskInput())).rejects.toThrow(SAVE_FAILURE_MESSAGE);
      expect(store.saved).toEqual([]);
    });

    it.each([0, -1, 1.5, Number.NaN])('rejects amount %p before any I/O', async (amountCents) => {
      const { features, store, useCase } = setup();
      await expect(useCase.execute(scoreRiskInput({ amountCents }))).rejects.toThrow(
        InvalidAssessmentInputError,
      );
      expect(features.queries).toEqual([]);
      expect(store.saved).toEqual([]);
    });
  });

  describe('test scenario gate', () => {
    it.each<[string, Setup]>([
      ['hooks default to off', {}],
      ['hooks explicitly off', { testHooksEnabled: false }],
    ])('ignores test_scenario when %s', async (_label, options) => {
      const { store, useCase } = setup(options);
      const out = await useCase.execute(scoreRiskInput({ testScenario: TestScenario.BLOCKED }));
      expect(out.decision).toBe(Decision.APPROVE);
      expect(out.reasons).toEqual([]);
      expect(store.saved[0]?.assessment.decision).toBe(Decision.APPROVE);
    });

    it('ignores even an unknown test_scenario value when hooks are off', async () => {
      const { useCase } = setup();
      // Simulates an unvalidated value from an untyped caller.
      const unknown = 'approve' as TestScenario;
      const out = await useCase.execute(scoreRiskInput({ testScenario: unknown }));
      expect(out.decision).toBe(Decision.APPROVE);
    });

    it.each<[TestScenario, Decision, number]>([
      [TestScenario.BLOCKED, Decision.REJECT, 70],
      [TestScenario.REVIEW, Decision.REVIEW, 30],
    ])(
      'raises a first-time card to the %s decision when hooks are on',
      async (scenario, decision, score) => {
        const { store, useCase } = setup({ testHooksEnabled: true });
        const out = await useCase.execute(scoreRiskInput({ testScenario: scenario }));
        expect(out.decision).toBe(decision);
        expect(out.score).toBe(score);
        expect(out.reasons.map((r) => r.code)).toEqual(['TEST_SCENARIO']);
        expect(store.saved[0]?.assessment.decision).toBe(decision);
        expect(store.saved[0]?.outboxEvent.payload).toMatchObject({ decision, score });
      },
    );

    it('never lets review downgrade a blocked card', async () => {
      const { features, useCase } = setup({ testHooksEnabled: true });
      features.set(firstTimeCardFeatures({ blocked: true }));
      const out = await useCase.execute(scoreRiskInput({ testScenario: TestScenario.REVIEW }));
      expect(out.decision).toBe(Decision.REJECT);
      expect(out.score).toBe(100);
      expect(out.reasons.map((r) => r.code)).toEqual(['BLOCKED_CARD', 'TEST_SCENARIO']);
    });

    it('keeps the rule verdict when hooks are on but no scenario is sent', async () => {
      const { useCase } = setup({ testHooksEnabled: true });
      const out = await useCase.execute(scoreRiskInput());
      expect(out.decision).toBe(Decision.APPROVE);
      expect(out.reasons).toEqual([]);
    });

    it('rejects an unknown test_scenario value when hooks are on', async () => {
      const { store, useCase } = setup({ testHooksEnabled: true });
      const unknown = 'approve' as TestScenario;
      await expect(useCase.execute(scoreRiskInput({ testScenario: unknown }))).rejects.toThrow(
        InvalidAssessmentInputError,
      );
      expect(store.saved).toEqual([]);
    });

    it('returns the stored assessment on a repeat even with a scenario', async () => {
      const { store, useCase } = setup({ testHooksEnabled: true });
      const first = await useCase.execute(scoreRiskInput());
      const repeat = await useCase.execute(scoreRiskInput({ testScenario: TestScenario.BLOCKED }));
      expect(repeat).toEqual({ ...first, deduplicated: true });
      expect(store.saved).toHaveLength(1);
    });
  });
});
