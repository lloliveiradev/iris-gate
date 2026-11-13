import { Decision } from '../domain/decision';
import { FraudReason } from '../domain/reason';
import { RiskAssessment, RiskAssessmentInput } from '../domain/risk-assessment';
import { DecisionThresholds } from '../domain/thresholds';
import { evaluateRules } from './evaluate-rules';
import { createFeatureSnapshot, FeatureSnapshot } from './feature-snapshot';
import {
  AccountRepo,
  Clock,
  IDGen,
  InboxStore,
  InvoiceHistoryRepo,
  InvoiceRecord,
  OutboxEventInput,
  RiskAssessmentRepo,
} from './ports';
import { RegisteredRule } from './rule-registry';

const FEATURE_WINDOW_MS = 24 * 60 * 60 * 1000;
/** Invoices averaged by the legacy bridge; the old `historyCount` default. */
const LEGACY_AVERAGE_SAMPLE_SIZE = 10;

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

    const now = this.clock.now();
    const features = await this.loadLegacyFeatures(input.accountRef, now);
    const { decision, score, reasons } = evaluateRules(
      this.rules,
      { features, amountCents: input.amountCents, now },
      this.thresholds,
    );

    const completedAt = this.clock.now();
    aggregate.complete(decision, score, reasons, completedAt);

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
        score,
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

    return { decision, score, reasons, deduplicated: false };
  }

  /**
   * Builds a `FeatureSnapshot` from the legacy account and invoice ports so the
   * Kafka path can run the phase 10 rules. The account ref stands in for the card
   * fingerprint, and declines are not tracked here.
   *
   * @deprecated Bridge for the legacy Kafka path only. Task 10.4 removes it together
   * with this use case, in favor of a `FeatureSnapshotReader`.
   */
  private async loadLegacyFeatures(accountRef: string, now: Date): Promise<FeatureSnapshot> {
    const account = await this.accounts.findOrCreate(accountRef);
    const since = new Date(now.getTime() - FEATURE_WINDOW_MS);
    const recent = await this.history.recentByAccount(accountRef, since);
    const lastInvoices = await this.history.lastNByAccount(accountRef, LEGACY_AVERAGE_SAMPLE_SIZE);
    return createFeatureSnapshot({
      cardFingerprint: accountRef,
      attempts24h: recent.length,
      declines24h: 0,
      avgCapturedCents: averageCents(lastInvoices),
      blocked: account.isSuspicious,
    });
  }
}

/** Average amount rounded to whole minor units, or `null` without invoices. */
function averageCents(invoices: readonly InvoiceRecord[]): number | null {
  if (invoices.length === 0) return null;
  const total = invoices.reduce((sum, invoice) => sum + invoice.amountCents, 0);
  return Math.round(total / invoices.length);
}
