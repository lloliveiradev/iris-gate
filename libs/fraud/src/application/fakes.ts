import { RiskAssessment, RiskAssessmentSnapshot } from '../domain/risk-assessment';
import {
  createFeatureSnapshot,
  FeatureSnapshot,
  FeatureSnapshotQuery,
  FeatureSnapshotReader,
} from './feature-snapshot';
import {
  AccountRepo,
  AccountSnapshot,
  Clock,
  DLQPublisher,
  EventPublisher,
  IDGen,
  InboxStore,
  InvoiceHistoryRepo,
  InvoiceRecord,
  OutboxEventInput,
  OutboxRecord,
  OutboxStore,
  RiskAssessmentRepo,
  RuleConfig,
  RuleConfigStore,
} from './ports';

export class InMemoryAccountRepo implements AccountRepo {
  private store = new Map<string, AccountSnapshot>();

  flag(ref: string, isSuspicious: boolean): void {
    this.store.set(ref, { ref, isSuspicious });
  }

  async findOrCreate(ref: string): Promise<AccountSnapshot> {
    const existing = this.store.get(ref);
    if (existing) return existing;
    const created: AccountSnapshot = { ref, isSuspicious: false };
    this.store.set(ref, created);
    return created;
  }
}

export class InMemoryInvoiceHistoryRepo implements InvoiceHistoryRepo {
  private invoices: InvoiceRecord[] = [];

  add(rec: InvoiceRecord): void {
    this.invoices.push(rec);
  }

  async recentByAccount(accountRef: string, since: Date): Promise<InvoiceRecord[]> {
    return this.invoices.filter(
      (i) => i.accountRef === accountRef && i.createdAt.getTime() > since.getTime(),
    );
  }

  async lastNByAccount(accountRef: string, limit: number): Promise<InvoiceRecord[]> {
    return this.invoices
      .filter((i) => i.accountRef === accountRef)
      .sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime())
      .slice(0, limit);
  }

  async recordAttempt(input: {
    paymentIntentId: string;
    accountRef: string;
    amountCents: number;
    occurredAt: Date;
  }): Promise<void> {
    this.invoices.push({
      id: input.paymentIntentId,
      accountRef: input.accountRef,
      amountCents: input.amountCents,
      createdAt: input.occurredAt,
    });
  }
}

export class InMemoryRiskAssessmentRepo implements RiskAssessmentRepo {
  readonly assessments = new Map<string, RiskAssessmentSnapshot>();
  readonly outbox: OutboxRecord[] = [];

  async findById(id: string): Promise<RiskAssessment | null> {
    const snap = this.assessments.get(id);
    return snap ? RiskAssessment.rehydrate(snap) : null;
  }

  async saveCompletedWithOutbox(
    assessment: RiskAssessment,
    outboxEvent: OutboxEventInput,
  ): Promise<void> {
    this.assessments.set(assessment.id, assessment.toSnapshot());
    this.outbox.push({
      eventId: outboxEvent.eventId,
      eventType: outboxEvent.eventType,
      topic: outboxEvent.topic,
      payload: outboxEvent.payload,
      occurredAt: outboxEvent.occurredAt,
      traceparent: outboxEvent.traceparent ?? null,
    });
  }

  saveFailedWithOutbox = this.saveCompletedWithOutbox.bind(this);
}

export class InMemoryInboxStore implements InboxStore {
  private reserved = new Map<string, string | null>();

  async reserve(messageId: string): Promise<{ accepted: boolean; assessmentId?: string }> {
    const existing = this.reserved.get(messageId);
    if (existing === undefined) {
      this.reserved.set(messageId, null);
      return { accepted: true };
    }
    return existing ? { accepted: false, assessmentId: existing } : { accepted: false };
  }

  async attachAssessment(messageId: string, assessmentId: string): Promise<void> {
    this.reserved.set(messageId, assessmentId);
  }
}

export class InMemoryOutboxStore implements OutboxStore {
  private records: OutboxRecord[] = [];

  push(rec: OutboxRecord): void {
    this.records.push(rec);
  }

  async pullUnpublished(limit: number): Promise<OutboxRecord[]> {
    return this.records.slice(0, limit);
  }

  async markPublished(eventId: string, _publishedAt: Date): Promise<void> {
    this.records = this.records.filter((r) => r.eventId !== eventId);
  }
}

export class CapturingEventPublisher implements EventPublisher {
  readonly published: OutboxRecord[] = [];

  async publish(record: OutboxRecord): Promise<void> {
    this.published.push(record);
  }
}

export class CapturingDLQ implements DLQPublisher {
  readonly entries: { topic: string; payload: unknown; reason: string }[] = [];

  async publish(input: { topic: string; payload: unknown; reason: string }): Promise<void> {
    this.entries.push(input);
  }
}

export class FixedClock implements Clock {
  constructor(private current: Date) {}

  advance(ms: number): void {
    this.current = new Date(this.current.getTime() + ms);
  }

  now(): Date {
    return this.current;
  }
}

export class SequentialIdGen implements IDGen {
  constructor(
    private counter = 0,
    private readonly prefix = 'gen',
  ) {}

  newId(): string {
    this.counter += 1;
    return `${this.prefix}_${String(this.counter).padStart(3, '0')}`;
  }
}

export class StaticRuleConfigStore implements RuleConfigStore {
  constructor(private readonly configs: RuleConfig[]) {}

  async load(): Promise<RuleConfig[]> {
    return [...this.configs];
  }
}

/** In-memory reader: unknown cards read as first-time cards. */
export class InMemoryFeatureSnapshotReader implements FeatureSnapshotReader {
  private readonly snapshots = new Map<string, FeatureSnapshot>();
  readonly queries: FeatureSnapshotQuery[] = [];

  set(snapshot: FeatureSnapshot): void {
    this.snapshots.set(snapshot.cardFingerprint, createFeatureSnapshot(snapshot));
  }

  async load(query: FeatureSnapshotQuery): Promise<FeatureSnapshot> {
    this.queries.push({ ...query });
    return (
      this.snapshots.get(query.cardFingerprint) ??
      createFeatureSnapshot({
        cardFingerprint: query.cardFingerprint,
        attempts24h: 0,
        declines24h: 0,
        avgCapturedCents: null,
        blocked: false,
      })
    );
  }
}
