import { RiskAssessment } from '../domain/risk-assessment';

export interface AccountSnapshot {
  ref: string;
  isSuspicious: boolean;
}

export interface InvoiceRecord {
  id: string;
  accountRef: string;
  amountCents: number;
  createdAt: Date;
}

export interface AccountRepo {
  findOrCreate(ref: string): Promise<AccountSnapshot>;
}

export interface InvoiceHistoryRepo {
  recentByAccount(accountRef: string, since: Date): Promise<InvoiceRecord[]>;
  lastNByAccount(accountRef: string, limit: number): Promise<InvoiceRecord[]>;
  recordAttempt(input: {
    paymentIntentId: string;
    accountRef: string;
    amountCents: number;
    occurredAt: Date;
  }): Promise<void>;
}

export interface RiskAssessmentRepo {
  findById(id: string): Promise<RiskAssessment | null>;
  saveCompletedWithOutbox(
    assessment: RiskAssessment,
    outboxEvent: OutboxEventInput,
  ): Promise<void>;
  saveFailedWithOutbox(
    assessment: RiskAssessment,
    outboxEvent: OutboxEventInput,
  ): Promise<void>;
}

export interface OutboxEventInput {
  eventId: string;
  eventType: string;
  topic: string;
  payload: Record<string, unknown>;
  occurredAt: Date;
  traceparent?: string;
}

export interface InboxStore {
  reserve(messageId: string): Promise<{ accepted: boolean; assessmentId?: string }>;
  attachAssessment(messageId: string, assessmentId: string): Promise<void>;
}

export interface OutboxStore {
  pullUnpublished(limit: number): Promise<OutboxRecord[]>;
  markPublished(eventId: string, publishedAt: Date): Promise<void>;
}

export interface OutboxRecord {
  eventId: string;
  eventType: string;
  topic: string;
  payload: Record<string, unknown>;
  occurredAt: Date;
  traceparent: string | null;
}

export interface EventPublisher {
  publish(record: OutboxRecord): Promise<void>;
}

export interface DLQPublisher {
  publish(input: { topic: string; payload: unknown; reason: string }): Promise<void>;
}

export interface Clock {
  now(): Date;
}

export interface IDGen {
  newId(): string;
}

export interface RuleConfig {
  code: string;
  weight: number;
  enabled: boolean;
  params: Record<string, number>;
}

export interface RuleConfigStore {
  load(): Promise<RuleConfig[]>;
}
