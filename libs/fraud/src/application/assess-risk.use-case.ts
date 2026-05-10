import { Decision } from '../domain/decision';
import { FraudReason, FraudReasonCode } from '../domain/reason';
import { RiskAssessment, RiskAssessmentInput } from '../domain/risk-assessment';
import { Score } from '../domain/score';
import { DecisionThresholds } from '../domain/thresholds';
import {
  AccountRepo,
  Clock,
  IDGen,
  InboxStore,
  InvoiceHistoryRepo,
  OutboxEventInput,
  RiskAssessmentRepo,
} from './ports';
import { RegisteredRule } from './rule-registry';

export interface AssessRiskInput {
  messageId: string;
  assessmentId: string;
  paymentIntentId: string;
  merchantId: string;
  accountRef: string;
  amountCents: number;
  currency: string;
  occurredAt: Date;
  traceparent?: string;
}

export interface AssessRiskOutput {
  decision: Decision;
  score: number;
  reasons: FraudReason[];
  deduplicated: boolean;
}

export class AssessRiskUseCase {
  constructor(
    private readonly inbox: InboxStore,
    private readonly accounts: AccountRepo,
    private readonly history: InvoiceHistoryRepo,
    private readonly assessments: RiskAssessmentRepo,
    private readonly rules: RegisteredRule[],
    private readonly thresholds: DecisionThresholds,
    private readonly clock: Clock,
    private readonly idGen: IDGen,
    private readonly outboxTopic = 'fraud.assessment.completed.v1',
  ) {}

  async execute(input: AssessRiskInput): Promise<AssessRiskOutput> {
    const reservation = await this.inbox.reserve(input.messageId);
    if (!reservation.accepted) {
      if (reservation.assessmentId) {
        const existing = await this.assessments.findById(reservation.assessmentId);
        if (existing) {
          const snap = existing.toSnapshot();
          return {
            decision: snap.decision ?? Decision.APPROVE,
            score: snap.score,
            reasons: [...snap.reasons],
            deduplicated: true,
          };
        }
      }
      return { decision: Decision.APPROVE, score: 0, reasons: [], deduplicated: true };
    }

    const aggregate = RiskAssessment.create({
      id: input.assessmentId,
      paymentIntentId: input.paymentIntentId,
      merchantId: input.merchantId,
      accountRef: input.accountRef,
      amountCents: input.amountCents,
      currency: input.currency,
      requestedAt: input.occurredAt,
    } satisfies RiskAssessmentInput);

    const account = await this.accounts.findOrCreate(input.accountRef);

    let total = Score.zero();
    const reasons: FraudReason[] = [];
    for (const { spec, weight } of this.rules) {
      const outcome = await spec.evaluate({
        account,
        amountCents: input.amountCents,
        paymentIntentId: input.paymentIntentId,
        history: this.history,
        now: this.clock.now(),
      });
      if (!outcome.matched) continue;
      const weighted = Score.fromWeighted(outcome.baseScore, weight);
      total = total.add(weighted);
      reasons.push({
        code: spec.code as FraudReasonCode,
        weight,
        score: outcome.baseScore,
        description: outcome.description,
      });
    }

    const decision = this.thresholds.classify(total.value);
    const completedAt = this.clock.now();
    aggregate.complete(decision, total.value, reasons, completedAt);

    const outboxEvent: OutboxEventInput = {
      eventId: this.idGen.newId(),
      eventType: this.outboxTopic,
      topic: this.outboxTopic,
      occurredAt: completedAt,
      traceparent: input.traceparent,
      payload: {
        event_id: this.idGen.newId(),
        occurred_at: completedAt.toISOString(),
        assessment_id: aggregate.id,
        payment_intent_id: aggregate.paymentIntentId,
        merchant_id: aggregate.merchantId,
        decision,
        score: total.value,
        reasons: reasons.map((r) => ({
          code: r.code,
          weight: r.weight,
          score: r.score,
          description: r.description,
        })),
      },
    };

    await this.assessments.saveCompletedWithOutbox(aggregate, outboxEvent);
    await this.history.recordAttempt({
      paymentIntentId: input.paymentIntentId,
      accountRef: input.accountRef,
      amountCents: input.amountCents,
      occurredAt: input.occurredAt,
    });
    await this.inbox.attachAssessment(input.messageId, aggregate.id);

    return { decision, score: total.value, reasons, deduplicated: false };
  }
}
