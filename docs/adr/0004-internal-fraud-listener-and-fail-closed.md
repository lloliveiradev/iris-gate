# ADR-0004 — Internal fraud listener and fail-closed risk scoring

- **Status:** Proposed
- **Date:** 2026-11-20
- **Deciders:** project owner
- **Related:** [`../specs/phase-10-sync-risk-scoring.md`](../specs/phase-10-sync-risk-scoring.md), [`../design/to-be.md`](../design/to-be.md) (decisions 3 and 8, sections 7, 8.6 and 11.3), [`../security.md`](../security.md), [`0002-outbox-inbox-for-kafka-atomicity.md`](0002-outbox-inbox-for-kafka-atomicity.md)

## Context

In the legacy architecture (`docs/design/as-is.md`, gap G1), `apps/api` and `apps/anti-fraud` were disconnected. Anti-fraud consumed `fraud.assessment.requested.v1` from Kafka, which nothing produced, while payment authorization ran without a risk check. Moving fraud scoring into the synchronous payment confirmation path (`docs/design/to-be.md` decisions 3 and 8, phase 10) creates two security and reliability boundaries:

1. **Internal API exposure.** `POST /internal/v1/risk-assessments` accepts a card fingerprint, amount, merchant id and optional `test_scenario`, writes to `payment_outcomes` and `RiskAssessment`, and produces `fraud.assessment.completed.v1`. Browsers and merchant clients must never reach this endpoint, and callers on the internal network must still authenticate.
2. **Uncertainty on the hot path.** Synchronous risk scoring sits in front of the acquirer with a tight latency budget (< 100 ms p99 locally, 300 ms hard client timeout). A slow database, network partition, bad configuration, integer overflow, or malformed response must never silently approve a payment.

Phase 10 left the listener isolation and service-authentication design to the implementer (spec item 3, gap G7). This ADR records the chosen boundary and fail-closed contract.

## Decision

`apps/anti-fraud` serves synchronous risk scoring on a dedicated internal HTTP listener protected by a shared service token, and every uncertain or failing path across both services fails closed.

### Separate internal HTTP listener

- `apps/anti-fraud` starts two NestJS HTTP applications from the same process (`apps/anti-fraud/src/main.ts`):
  - The **public listener** (`PORT`, default `13001` on host, `3001` inside Compose) serves only `/healthz` and `/readyz`. `RiskAssessmentController` is never mounted on `AppModule`.
  - The **internal listener** (`RiskInternalModule`, built by `createInternalApp`) serves `POST /internal/v1/risk-assessments` (`libs/contracts/http/fraud-internal.openapi.yaml`) on `ANTI_FRAUD_INTERNAL_HOST` (default `127.0.0.1`) and `ANTI_FRAUD_INTERNAL_PORT` (default `13002`).
- Boot fails if `ANTI_FRAUD_INTERNAL_PORT` is invalid or equals `PORT`.
- In local Docker Compose (`infra/docker/docker-compose.yml`), the container sets `ANTI_FRAUD_INTERNAL_HOST=0.0.0.0` so Docker bridge forwarding works, while the host port mapping binds loopback only: `127.0.0.1:${ANTI_FRAUD_INTERNAL_PORT:-13002}:13002`.

### Service-token authentication

- `ServiceTokenGuard` (`apps/anti-fraud/src/modules/risk/service-token.guard.ts`) protects `POST /internal/v1/risk-assessments` and requires `Authorization: Bearer <FRAUD_SERVICE_TOKEN>`.
- Tokens are compared with `crypto.timingSafeEqual` over equal-length UTF-8 buffers so neither content nor length leaks via timing.
- Missing, malformed, or wrong tokens answer `401` with RFC 7807 `application/problem+json` (`ProblemExceptionFilter`). Neither the expected nor the provided token is logged.
- `loadRiskConfig` fails fast at startup when `FRAUD_SERVICE_TOKEN` is unset, empty, or whitespace-only.

### Fail-closed scoring and client boundary

- **Go caller (`libs/payments/application/risk_assessor.go`, `libs/payments/infra/riskclient/client.go`):**
  - `RiskAssessor.Assess(ctx, req)` returns `RiskSnapshot` with no `error` return value, preventing callers from ignoring an error or falling back to `APPROVE`.
  - `riskclient.Client` enforces a `300 ms` total timeout (`Timeout`), performs zero retries, refuses HTTP redirects, bounds response reads to `64 KiB`, and validates every contract field (`assessment_id`, `decision`, `score` in `0..100`, known `reason.code`, no trailing JSON).
  - Any timeout, network error, canceled context, non-2xx status, or invalid response returns `RiskUnavailableSnapshot()` (`Decision: REVIEW`, `Reasons: ["RISK_UNAVAILABLE"]`) and logs the structured `cause` without card data or the service token.
- **Anti-fraud engine (`libs/fraud`, `apps/anti-fraud`):**
  - `buildRulesFromConfig` and `assertBlockedCardRejects` fail startup if `BLOCKED_CARD` is missing, disabled, or weighted below the reject threshold, or if any configured rule has invalid parameters.
  - `ScoreRiskUseCase` validates input and `FeatureSnapshot` before scoring and propagates feature-reader, persistence, or incomplete-assessment errors rather than returning `APPROVE`.
  - `PrismaScoredAssessmentStore` writes the assessment (with `features_snapshot`), the `attempted` row in `payment_outcomes`, and the `fraud.assessment.completed.v1` outbox row in one Prisma `$transaction`. Repeats for the same `payment_intent_id` return the stored assessment; conflicting `merchant_id`, `amount_cents`, or `currency` returns `409` (`application/problem+json`).
  - `bigint.mapping.ts` converts Postgres `BIGINT` amounts and counts with explicit safe-integer bounds checks and throws `UnsafeIntegerError` outside `Number.MIN_SAFE_INTEGER..Number.MAX_SAFE_INTEGER` instead of rounding.
  - `FRAUD_TEST_HOOKS` defaults to `false` in code (`risk.config.ts`, `ScoreRiskUseCase`); `test_scenario` is ignored unless explicitly enabled (`true` in local `.env.example` and local Compose). Even when enabled, `applyTestScenario` can only escalate severity (`REVIEW` or `REJECT`), never downgrade a stricter rule verdict.

## Alternatives considered

### A. A path on the public listener

Mounting `POST /internal/v1/risk-assessments` on the existing public port (`13001` / `3001`) behind `ServiceTokenGuard` or reverse-proxy path rules would avoid starting a second Nest HTTP server. We rejected it because a single proxy misconfiguration, ingress wildcard, or route refactor could expose the internal scoring path to browsers and external callers. Listener separation enforces isolation at the socket and port-mapping layer independently of HTTP routing.

### B. mTLS between services

Mutual TLS between `apps/api` and `apps/anti-fraud` would authenticate both peers at the transport layer and encrypt traffic in transit. Iris Gate already plans a local dev CA and mTLS for the card vault in phase 14 (`docs/design/to-be.md` section 10). Adding certificate issuance, trust stores, and rotation across Go and NestJS in phase 10 would duplicate phase 14's PKI work before the vault exists. Bearer service tokens over a loopback/private listener provide proportional protection now and can later sit behind mTLS or service-mesh TLS without changing `fraud-internal.openapi.yaml`.

### C. Kafka request and reply (retired in phase 10)

The legacy design put `fraud.assessment.requested.v1` on Kafka and waited for `fraud.assessment.completed.v1`. We retired that flow (`docs/design/to-be.md` decision 3 and section 7) because Kafka moves facts that already happened, not synchronous RPCs. A broker round-trip, outbox polling delay, consumer poll interval, and reply-correlation state inside payment confirmation cannot reliably meet the `< 100 ms` p99 local target or the `300 ms` timeout budget. `fraud.assessment.completed.v1` stays on Kafka via the anti-fraud outbox for asynchronous consumers (audit log, analytics).

### D. A network policy only

Relying solely on `127.0.0.1` binding or Docker/VPC network policies without `ServiceTokenGuard` would remove token management. We rejected it because defense in depth requires the service to authenticate every caller regardless of network position. In Docker Compose, the anti-fraud container binds `0.0.0.0:13002` inside the `ig` bridge network so host loopback forwarding reaches it; without `ServiceTokenGuard`, any other container on that network or an SSRF bug could invoke scoring and pollute `payment_outcomes`.

## Consequences

### Positive

- Public health probes and internal risk scoring have separate ports and Nest application trees; `/internal/v1/risk-assessments` answers `404` on the public listener.
- Missing credentials or broken rule configuration fail fast at boot rather than silently weakening fraud controls.
- Any scoring outage, timeout, or contract violation holds payments for `REVIEW` (`RISK_UNAVAILABLE`) instead of approving them or blocking checkout with a hard 500.
- The OpenAPI contract (`fraud-internal.openapi.yaml`) keeps Go and TypeScript wire shapes aligned.

### Negative

- Running two Nest HTTP servers in one process requires managing a second port (`ANTI_FRAUD_INTERNAL_PORT`) and shutdown hook.
- Local development uses a static shared secret (`FRAUD_SERVICE_TOKEN`); production deployments must store it in a secrets manager and rotate it, or upgrade transport to mTLS.
- `FRAUD_TEST_HOOKS` defaults to `true` in local `.env.example` and `docker-compose.yml` for testability, so any non-local environment must explicitly keep it unset or `false`.
- Fail-closed `REVIEW` on outages increases manual review volume if `apps/anti-fraud` becomes slow or unavailable.

### Metrics to expose

- `risk_assessment_latency_ms` (histogram, target p99 < 100 ms locally)
- `risk_assessment_decisions_total` (counter, labeled by `decision`)
- `riskclient_fallback_total` (counter, labeled by `cause`: `timeout`, `network_error`, `non_2xx_status`, `invalid_body`)
- `risk_assessment_auth_failures_total` (counter)
