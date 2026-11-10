# Legacy repositories audit (historical)

> **Historical document.** This audit was done on 2026-05-09 on the three repositories that preceded the Iris Gate monorepo. Those repositories have since been deleted. It is kept to explain where the project started and why some decisions were made (see ADR-0002 and ADR-0003). For the current state of the code, read [`design/as-is.md`](design/as-is.md).

Each section lists what was **reused as a concept** (not copied) and what was **discarded or rewritten**.

## 1. `go-gateway` (Go, chi, Postgres, Kafka)

### Structure

```text
cmd/app/main.go              entry point
internal/domain/             Account, Invoice, CreditCard, events
internal/service/            AccountService, InvoiceService, Kafka producer and consumer
internal/repository/         raw SQL repositories
internal/web/handlers/       AccountHandler, InvoiceHandler
internal/web/middleware/     AuthMiddleware (X-API-Key)
internal/dto/                request and response shapes
migrations/                  *.up.sql, no runner
```

### Domain model

- **Account:** `id, name, email, api_key, balance, timestamps`. The key was generated with `crypto/rand` but stored in plain text.
- **Invoice:** `id, account_id, amount, status {pending, approved, rejected}, payment_type, card_last_digits`. `Process()` approved invoices below 10,000 with `rand.Float64()`; larger ones stayed pending for Kafka to decide.
- **CreditCard:** only the last four digits were stored, but the full number travelled in plain text from handler to domain.

### Kafka

Invoices of 10,000 or more were published to `pending_transactions`; results came back on `transaction_results`. The consumer loop had no cancellation, no signal handling and no graceful shutdown.

### Findings

| Severity | Problem |
| --- | --- |
| Critical | PAN in memory in plain text across layers |
| Critical | API keys stored in plain text |
| Critical | No idempotency on invoice creation |
| Critical | Save then publish to Kafka, not atomic: a failed publish left a permanent inconsistency |
| High | Approval decided by `rand.Float64()` |
| High | No tests at all |
| High | No migration runner |
| Medium | Domain coupled to infrastructure types; every service error mapped to HTTP 500; no request id, traces or metrics |

### Verdict

Reused as concepts: the layering, API key middleware, the producer and consumer skeleton. Rewritten: card handling (tokenization), credentials (Argon2id), atomicity (outbox), approval (delegated to the fraud engine), a domain free of infrastructure.

## 2. `anti-fraud` (NestJS, Prisma, KafkaJS)

### Structure and schema

```text
src/invoices/                controller, service, consumer
src/invoices/fraud/          fraud service and specifications
src/kafka/                   custom Nest KafkaServer, KafkaService
prisma/schema.prisma         Account, Invoice, FraudHistory
```

```text
Account      { id, isSuspicious, timestamps, invoices[] }
Invoice      { id, accountId, amount: Float, status: APPROVED | REJECTED, fraudHistory? }
FraudHistory { id, invoiceId (unique), reason, description, timestamps }
```

Missing: a numeric score, rule weights, a snapshot of the evaluated features, a persistent blocklist.

### Fraud engine: the best asset

A specification pattern with an aggregator: `SuspiciousAccountSpecification`, `UnusualAmountSpecification` (amount above average by a threshold), `FrequentHighValueSpecification` (count in a time window). The first matching rule won; there was no score composition. Adding a rule meant adding a class and registering it. **This pattern was kept** and extended with weighted scores in phase 4.

### Findings

| Severity | Problem |
| --- | --- |
| Critical | Consumer not idempotent: a redelivery duplicated the decision |
| Critical | No authentication between services: anyone reaching the broker could publish transactions |
| High | No DLQ, no retry with backoff, no circuit breaker; a Kafka failure crashed the app at startup |
| High | No global validation pipe: DTO decorators were never enforced |
| Medium | Controllers called Prisma directly; binary scores; tests only checked `toBeDefined()` |

### Verdict

Reused: the specification pattern, the custom Kafka server idea, the schema as a starting point. Rewritten: inbox-based idempotent consumer, weighted and versioned scores, validated DTOs, rule weights in the database.

## 3. `gateway-web` (Next 15, shadcn/ui, React Hook Form, Zod)

### Structure and stack

Next 15.2, React 19, Tailwind 3.4, shadcn/ui, React Hook Form, Zod, Sonner, next-themes. Pages for login (API key in a cookie), invoice list, new invoice with a card form, an empty invoice detail page and an empty PDF route. Backend types were duplicated by hand in `interfaces/Invoice.ts`.

### Findings

| Severity | Problem |
| --- | --- |
| Critical | The login sent the API key from the browser, visible in devtools; the API URL was public |
| Critical | The `apiKey` cookie had no `httpOnly`, `Secure` or `SameSite=Strict` |
| High | Backend types duplicated by hand, so drift was guaranteed; no controlled cache revalidation |
| Medium | Empty detail and PDF routes; generic untyped fetch errors; no loading states |

### Verdict

Reused: shadcn/ui setup, Tailwind config, the server-action pattern. Rewritten: a typed server-side client (`libs/sdk-web`), types generated from OpenAPI, a server-side encrypted session so credentials never reach the browser.

## 4. Cross-repository summary

The three repositories modelled the same domain differently: `Account.isSuspicious` existed only in anti-fraud; invoice status was `pending|approved|rejected` in Go, `APPROVED|REJECTED` in Nest and a free string in the web app; the Kafka payloads disagreed on field names and casing. The monorepo started by **consolidating contracts** in `libs/contracts` (ADR-0003) and moving every cross-service message to the outbox and inbox (ADR-0002).
