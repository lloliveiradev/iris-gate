# Iris Gate: as-is (current state)

> Snapshot of what the code does on 2026-11-10 (commit `7195707`). Every claim here was read from the source, not from older docs. Where `docs/architecture.md` or `docs/roadmap.md` disagree, this file and the code win.
>
> The target design is in [`to-be.md`](to-be.md).

## 1. Summary

- Three processes: `apps/api` (Go), `apps/anti-fraud` (NestJS worker) and `apps/web` (Next.js dashboard and landing page).
- Two Postgres databases: `iris` (identity and payments share it) and `antifraud`.
- Kafka carries events through a transactional outbox on both sides. The anti-fraud consumer deduplicates with an inbox.
- **The payment flow and the fraud engine are not connected.** Payments publishes `payments.payment_intent.created.v1`; anti-fraud only consumes `fraud.assessment.requested.v1`, which nothing produces.
- **Authorization and capture are manual.** A new intent stays `pending` until someone clicks **Authorize** and then **Capture** on the dashboard detail page. No risk check runs before the acquirer call.

## 2. Components and data flow

```mermaid
flowchart LR
  subgraph Browser
    U[Merchant user]
  end

  subgraph web[apps/web - Next.js 16]
    LP[Landing page]
    DB[Dashboard<br/>server actions + iron-session]
  end

  subgraph api[apps/api - Go composition root]
    ID[libs/identity<br/>merchants, API keys]
    PAY[libs/payments<br/>payment intents]
    IDEM[idempotency middleware]
    RELAY[outbox relay goroutine]
    ACQ[FakeAcquirer<br/>always approves]
  end

  subgraph af[apps/anti-fraud - NestJS worker]
    CONS[AssessmentConsumer]
    UC[AssessRiskUseCase<br/>libs/fraud rules]
    AFRELAY[OutboxRelayService]
    DLQ[KafkaDLQPublisher]
  end

  PG1[(Postgres: iris)]
  PG2[(Postgres: antifraud)]
  K{{Kafka KRaft<br/>+ Karapace}}

  U --> LP
  U --> DB
  DB -- "sdk-web, Bearer irk_" --> api
  IDEM --> PAY
  ID --> PG1
  PAY --> PG1
  PAY -- "Authorize (on button click)" --> ACQ
  RELAY -- "poll outbox_messages" --> PG1
  RELAY -- "payments.payment_intent.*.v1" --> K

  K -- "fraud.assessment.requested.v1<br/>(no producer exists)" -.-> CONS
  CONS --> UC
  UC --> PG2
  AFRELAY -- "poll OutboxMessage" --> PG2
  AFRELAY -- "fraud.assessment.completed.v1<br/>(no consumer exists)" --> K
  CONS -- "invalid payloads" --> DLQ --> K
```

Dashed edge: wired on the consumer side only. Nothing in the repo writes that topic, and nothing reads `fraud.assessment.completed.v1`.

## 3. Payment sequence today

```mermaid
sequenceDiagram
  autonumber
  actor M as Merchant (dashboard)
  participant W as apps/web
  participant A as apps/api
  participant DB as Postgres iris
  participant ACQ as FakeAcquirer
  participant R as Outbox relay
  participant K as Kafka

  M->>W: New payment form (amount, currency, card_token typed by hand)
  W->>A: POST /v1/payment-intents (Idempotency-Key)
  A->>DB: one tx: insert payment_intents(status=pending) + outbox row
  A-->>W: 201 pending
  R->>DB: pull unpublished
  R->>K: payments.payment_intent.created.v1
  Note over K: No consumer. Anti-fraud never sees it.

  M->>W: clicks "Authorize"
  W->>A: POST /v1/payment-intents/{id}/authorize
  A->>DB: GetByID (no lock, no tenant check)
  A->>ACQ: Authorize(card_token, amount)
  ACQ-->>A: approved
  A->>DB: one tx: update status=authorized + outbox row
  A-->>W: 200 authorized

  M->>W: clicks "Capture"
  W->>A: POST /v1/payment-intents/{id}/capture
  A->>DB: update status=captured + outbox row
  A-->>W: 200 captured
```

## 4. Payment intent state machine

```mermaid
stateDiagram-v2
  [*] --> pending: POST /v1/payment-intents
  pending --> authorized: POST .../authorize (manual)
  authorized --> captured: POST .../capture (manual)
  pending --> failed: acquirer declined (in memory only, see D3)
  captured --> [*]
  failed --> [*]
  note right of pending
    refunded and canceled exist in the
    DB CHECK and the Go enum, but no
    method or endpoint reaches them.
  end note
```

## 5. Data model

### 5.1 Database `iris` (identity + payments)

```mermaid
erDiagram
  merchants ||--o{ api_keys : "issues"
  merchants ||--o{ payment_intents : "owns (cross-context FK)"

  merchants {
    uuid id PK
    text legal_name
    varchar14 document UK "14 digits"
    citext contact_email
    text status "pending | active | suspended"
    timestamptz created_at
    timestamptz updated_at
  }
  api_keys {
    uuid id PK
    uuid merchant_id FK
    varchar32 prefix UK
    text hash "Argon2id PHC"
    text status "active | revoked"
    text_array scopes
    timestamptz created_at
    timestamptz last_used_at
    timestamptz revoked_at
  }
  payment_intents {
    uuid id PK
    uuid merchant_id FK
    bigint amount_cents "> 0"
    char3 currency "ISO-4217"
    text status "pending | authorized | captured | refunded | canceled | failed"
    text card_token "opaque, 8..256"
    text description
    timestamptz created_at
    timestamptz authorized_at
    timestamptz captured_at
  }
  outbox_messages {
    uuid id PK
    text topic
    text msg_key "merchant_id today"
    jsonb payload
    jsonb headers
    timestamptz created_at
    timestamptz published_at "NULL = pending"
  }
  idempotency_records {
    text scope PK "route name, not merchant"
    text key PK
    bytea request_hash
    int response_status
    bytea response_body
    timestamptz created_at
  }
```

Not stored: the acquirer reference (`acquirer_ref` exists only in the authorized event), failure reason, capture method, risk decision, and any row version.

### 5.2 Database `antifraud`

```mermaid
erDiagram
  Account ||..o{ InvoiceAttempt : "accountRef (no FK)"
  Account ||..o{ RiskAssessment : "accountRef (no FK)"
  InboxMessage |o..o| RiskAssessment : "assessmentId"

  Account {
    string ref PK
    boolean isSuspicious
    datetime createdAt
    datetime updatedAt
  }
  InvoiceAttempt {
    string paymentIntentId PK
    string accountRef
    int amountCents "Int32"
    datetime occurredAt
  }
  RiskAssessment {
    string id PK
    string paymentIntentId UK
    string merchantId
    string accountRef
    int amountCents "Int32"
    char3 currency
    string status "PENDING | COMPLETED | FAILED"
    string decision "APPROVE | REVIEW | REJECT"
    float score "0..100"
    json reasons
    datetime requestedAt
    datetime completedAt
  }
  FraudRule {
    string code PK
    float weight
    boolean enabled
    json params
  }
  InboxMessage {
    string messageId PK
    string topic
    string assessmentId
    datetime receivedAt
    datetime processedAt
  }
  OutboxMessage {
    string eventId PK
    string eventType
    string topic
    json payload
    string traceparent
    datetime occurredAt
    datetime publishedAt
  }
```

## 6. Classes

### 6.1 Identity (Go, `libs/identity`)

```mermaid
classDiagram
  direction LR
  class Merchant {
    +UUID ID
    +string LegalName
    +string Document
    +string ContactEmail
    +MerchantStatus Status
  }
  class APIKey {
    +UUID ID
    +UUID MerchantID
    +string Prefix
    +string Hash
    +APIKeyStatus Status
  }
  class CreateMerchantUseCase
  class IssueAPIKeyUseCase
  class AuthenticateAPIKeyUseCase
  class RevokeAPIKeyUseCase
  class MerchantRepo {
    <<interface>>
  }
  class APIKeyRepo {
    <<interface>>
  }
  class Hasher {
    <<interface>>
  }
  class SecretGen {
    <<interface>>
  }
  Merchant "1" --> "*" APIKey
  CreateMerchantUseCase ..> MerchantRepo
  IssueAPIKeyUseCase ..> APIKeyRepo
  IssueAPIKeyUseCase ..> Hasher
  IssueAPIKeyUseCase ..> SecretGen
  AuthenticateAPIKeyUseCase ..> APIKeyRepo
  AuthenticateAPIKeyUseCase ..> Hasher
  RevokeAPIKeyUseCase ..> APIKeyRepo
```

### 6.2 Payments (Go, `libs/payments`)

```mermaid
classDiagram
  direction LR
  class PaymentIntent {
    +UUID ID
    +UUID MerchantID
    +Money Amount
    +PaymentIntentStatus Status
    +string CardToken
    +string Description
    +time CreatedAt
    +time* AuthorizedAt
    +time* CapturedAt
    +MarkAuthorized(at) error
    +MarkCaptured(at) error
    +MarkFailed() error
  }
  class Money {
    +int64 AmountCents
    +string Currency
  }
  class CreatePaymentIntentUseCase
  class AuthorizePaymentUseCase
  class CapturePaymentUseCase
  class PaymentIntentRepo {
    <<interface>>
    +CreateWithOutbox(p, ev)
    +UpdateWithOutbox(p, ev)
    +GetByID(id)
    +ListByMerchant(merchantID, limit)
    +GetByMerchantAndIdempotencyKey() stub
  }
  class Acquirer {
    <<interface>>
    +Authorize(in) AuthorizeOutput
  }
  class FakeAcquirer
  class HTTPAcquirer
  class Relay {
    +Run(ctx)
  }
  PaymentIntent *-- Money
  CreatePaymentIntentUseCase ..> PaymentIntentRepo
  AuthorizePaymentUseCase ..> PaymentIntentRepo
  AuthorizePaymentUseCase ..> Acquirer
  CapturePaymentUseCase ..> PaymentIntentRepo
  Acquirer <|.. FakeAcquirer
  Acquirer <|.. HTTPAcquirer
```

The `Acquirer` port has no `Capture`, `Void` or `Refund`: capture is a local status change only.

### 6.3 Fraud (TypeScript, `libs/fraud` + `apps/anti-fraud`)

```mermaid
classDiagram
  direction LR
  class RiskAssessment {
    +string id
    +string paymentIntentId
    +AssessmentStatus status
    +Decision decision
    +number score
    +FraudReason[] reasons
    +create(input)$ RiskAssessment
    +complete(decision, score, reasons, at)
    +fail(at)
  }
  class Score {
    +number value
    +add(other) Score
    +fromWeighted(base, weight)$ Score
  }
  class DecisionThresholds {
    +number review
    +number reject
    +defaults()$ DecisionThresholds
    +classify(score) Decision
  }
  class FraudSpecification {
    <<interface>>
    +code
    +evaluate(ctx) RuleOutcome
  }
  class SuspiciousAccountSpec
  class UnusualAmountSpec
  class FrequentHighValueSpec
  class AssessRiskUseCase {
    +execute(input) AssessRiskOutput
  }
  class AssessmentConsumer
  class OutboxRelayService
  FraudSpecification <|.. SuspiciousAccountSpec
  FraudSpecification <|.. UnusualAmountSpec
  FraudSpecification <|.. FrequentHighValueSpec
  AssessRiskUseCase ..> FraudSpecification
  AssessRiskUseCase ..> RiskAssessment
  AssessRiskUseCase ..> Score
  AssessRiskUseCase ..> DecisionThresholds
  AssessmentConsumer --> AssessRiskUseCase
```

## 7. Kafka topics today

| Topic | Producer | Consumer | Key |
| --- | --- | --- | --- |
| `identity.merchant.created.v1` | none (schema only) | none | n/a |
| `payments.payment_intent.created.v1` | payments outbox relay | none | merchant_id |
| `payments.payment_intent.authorized.v1` | payments outbox relay | none | merchant_id |
| `payments.payment_intent.captured.v1` | payments outbox relay | none | merchant_id |
| `fraud.assessment.requested.v1` | **none** | anti-fraud | payment_intent_id |
| `fraud.assessment.completed.v1` | anti-fraud outbox relay | none | payment_intent_id |
| `fraud.assessment.failed.dlq` | anti-fraud consumer | none | n/a |

## 8. Gaps and defects

### 8.1 Defects (bugs in code that exists)

| ID | Severity | Defect | Where |
| --- | --- | --- | --- |
| D1 | Critical | **Cross-tenant authorize and capture.** `handleAuthorize` and `handleCapture` load the intent by id without checking `pi.MerchantID` against the authenticated merchant. Any valid API key can authorize or capture another merchant's intent. `GET /v1/payment-intents/{id}` does check. | `libs/payments/infra/httpapi/handlers.go` |
| D2 | Critical | **Idempotency keys are global, not per merchant.** The scope is the constant `create_payment_intent`. Two merchants that send the same key and body get the first merchant's stored response, including its `card_token`; a different body returns 409. | `apps/api/cmd/api/main.go:100`, `idempotency_records` PK |
| D3 | High | **Acquirer decline is not persisted.** `AuthorizePaymentUseCase` calls `MarkFailed()` in memory and returns; the row stays `pending` and no event is written. | `libs/payments/application/authorize_payment.go` |
| D4 | High | **Double authorization under concurrency.** Authorize reads, calls the acquirer, then updates with no row lock, version check or conditional `WHERE status = ...`. Two concurrent calls both hit the acquirer. | `authorize_payment.go`, `pgstore.UpdateWithOutbox` |
| D5 | Medium | `acquirer_ref` is never stored, so a later capture, void or refund cannot reference the authorization. | `payment_intents` schema |
| D6 | Medium | Anti-fraud stores amounts as Prisma `Int` (32-bit). Amounts above 21,474,836.47 overflow, while payments uses `int64`. | `apps/anti-fraud/prisma/schema.prisma` |
| D7 | Low | `GetByMerchantAndIdempotencyKey` is a stub that always returns not found. | `libs/payments/infra/pgstore/pgstore.go` |
| D8 | Critical | **Identity bootstrap endpoints are public.** `POST /v1/merchants`, `POST /v1/api-keys` and `POST /v1/api-keys/{id}/revoke` have no authentication ("hardening in phase 7"). Anyone who knows a merchant id can issue a working API key for that merchant, which is a full account takeover, and anyone can revoke any key. | `libs/identity/infra/httpapi/handlers.go` (`Mount`) |
| D9 | Medium | **The dashboard logs in with the merchant's secret API key** and keeps it in the session cookie. There are no user accounts, so every dashboard action is attributed to "the merchant", and a leaked session is a leaked integration key. | `apps/web/src/lib/session.ts`, `apps/web/src/app/[locale]/login/actions.ts` |

### 8.2 Missing behavior (gaps against a market-standard gateway)

| ID | Gap |
| --- | --- |
| G1 | No risk check before authorization. Payments and fraud are not connected at all. |
| G2 | Authorization is triggered by a dashboard button, not by a customer checkout. Capture is always manual; there is no automatic capture and no per-merchant capture setting. |
| G3 | No checkout or tokenization. The card token is typed by hand into the dashboard form. |
| G4 | No cancel (void), refund, authorization expiry, or review queue. |
| G5 | The acquirer port only authorizes. The fake acquirer always approves, so declines cannot be demonstrated. |
| G6 | Fraud history (`InvoiceAttempt`) is only written by the assessment request itself; real payment outcomes (authorized, declined, captured, refunded) never reach the fraud engine. |
| G7 | Payments has no inbox, so it cannot safely consume events. |
| G8 | `payment_intents.merchant_id` is a foreign key into identity's table, against the rule that contexts do not share tables. |
| G9 | Playwright covers only unauthenticated pages. There is no end-to-end test of a payment. |
| G10 | Visual identity: the green palette is used for backgrounds, borders and muted text, so large surfaces read as tinted green. |
