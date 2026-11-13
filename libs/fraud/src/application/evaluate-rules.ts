import { assertAmountCents } from '../domain/amount';
import { Decision } from '../domain/decision';
import { FraudReason } from '../domain/reason';
import { Score } from '../domain/score';
import { DecisionThresholds } from '../domain/thresholds';
import { createFeatureSnapshot } from './feature-snapshot';
import { RegisteredRule } from './rule-registry';
import { RuleContext } from './specifications/rule';

/** Result of running the rules over one payment. */
export interface RuleEvaluation {
  decision: Decision;
  score: number;
  reasons: FraudReason[];
}

/**
 * Runs every registered rule over the context, sums the weighted scores (capped at
 * 100) and classifies the total. The amount and the snapshot are validated first, so
 * malformed input throws instead of approving.
 */
export function evaluateRules(
  rules: readonly RegisteredRule[],
  ctx: RuleContext,
  thresholds: DecisionThresholds,
): RuleEvaluation {
  assertAmountCents(ctx.amountCents);
  const checked: RuleContext = { ...ctx, features: createFeatureSnapshot(ctx.features) };

  let total = Score.zero();
  const reasons: FraudReason[] = [];
  for (const { spec, weight } of rules) {
    const outcome = spec.evaluate(checked);
    if (!outcome.matched) continue;
    total = total.add(Score.fromWeighted(outcome.baseScore, weight));
    reasons.push({
      code: spec.code,
      weight,
      score: outcome.baseScore,
      description: outcome.description,
    });
  }

  return { decision: thresholds.classify(total.value), score: total.value, reasons };
}
