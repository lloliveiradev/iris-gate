import { RiskAssessment } from '../domain/risk-assessment';

export interface RiskAssessmentRepo {
  findById(id: string): Promise<RiskAssessment | null>;
  saveCompletedWithOutbox(assessment: RiskAssessment, outboxEvent: OutboxEventInput): Promise<void>;
  saveFailedWithOutbox(assessment: RiskAssessment, outboxEvent: OutboxEventInput): Promise<void>;
}

export interface OutboxEventInput {
  eventId: string;
  eventType: string;
  topic: string;
  payload: Record<string, unknown>;
  occurredAt: Date;
  traceparent?: string;
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
