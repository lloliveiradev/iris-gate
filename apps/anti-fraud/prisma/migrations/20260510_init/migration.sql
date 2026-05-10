-- Anti-fraud initial schema (Phase 4).

CREATE TABLE "Account" (
  "ref"          TEXT PRIMARY KEY,
  "isSuspicious" BOOLEAN NOT NULL DEFAULT false,
  "createdAt"    TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt"    TIMESTAMP(3) NOT NULL
);

CREATE TABLE "InvoiceAttempt" (
  "paymentIntentId" TEXT PRIMARY KEY,
  "accountRef"      TEXT NOT NULL,
  "amountCents"     INTEGER NOT NULL,
  "occurredAt"      TIMESTAMP(3) NOT NULL,
  "createdAt"       TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX "InvoiceAttempt_accountRef_occurredAt_idx"
  ON "InvoiceAttempt" ("accountRef", "occurredAt");

CREATE TABLE "RiskAssessment" (
  "id"              TEXT PRIMARY KEY,
  "paymentIntentId" TEXT NOT NULL UNIQUE,
  "merchantId"      TEXT NOT NULL,
  "accountRef"      TEXT NOT NULL,
  "amountCents"     INTEGER NOT NULL,
  "currency"        CHAR(3) NOT NULL,
  "status"          TEXT NOT NULL,
  "decision"        TEXT,
  "score"           DOUBLE PRECISION NOT NULL DEFAULT 0,
  "reasons"         JSONB NOT NULL DEFAULT '[]'::jsonb,
  "requestedAt"     TIMESTAMP(3) NOT NULL,
  "completedAt"    TIMESTAMP(3),
  "createdAt"       TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt"       TIMESTAMP(3) NOT NULL
);
CREATE INDEX "RiskAssessment_accountRef_completedAt_idx"
  ON "RiskAssessment" ("accountRef", "completedAt");

CREATE TABLE "FraudRule" (
  "code"      TEXT PRIMARY KEY,
  "weight"    DOUBLE PRECISION NOT NULL,
  "enabled"   BOOLEAN NOT NULL DEFAULT true,
  "params"    JSONB NOT NULL DEFAULT '{}'::jsonb,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL
);

CREATE TABLE "InboxMessage" (
  "messageId"    TEXT PRIMARY KEY,
  "topic"        TEXT NOT NULL,
  "assessmentId" TEXT,
  "receivedAt"   TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "processedAt"  TIMESTAMP(3)
);

CREATE TABLE "OutboxMessage" (
  "eventId"     TEXT PRIMARY KEY,
  "eventType"   TEXT NOT NULL,
  "topic"       TEXT NOT NULL,
  "payload"     JSONB NOT NULL,
  "traceparent" TEXT,
  "occurredAt"  TIMESTAMP(3) NOT NULL,
  "publishedAt" TIMESTAMP(3),
  "createdAt"   TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX "OutboxMessage_publishedAt_idx" ON "OutboxMessage" ("publishedAt");
