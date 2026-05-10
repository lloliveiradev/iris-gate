# Roadmap — Do Esqueleto ao MVP

> Estimativas em "semanas-pessoa" assumindo 1 dev sênior FTE. Paralelizável onde indicado.

---

## Fase 0 — Fundação (semana 1)

**Objetivo:** monorepo executável. Hoje não é.

> **Deltas vs. plano original** (decididos durante implementação):
> - Gerenciador de pacotes: **Yarn 4** (Berry, `nodeLinker: node-modules`) — não pnpm.
> - Orquestração local: **Makefiles** (raiz delegando + um por galho) — não `bootstrap.sh`.
> - Mensageria local: **Apache Kafka latest (KRaft, single node)** — não Redpanda. Trocável depois.
> - Banco local: **Postgres 18.3-alpine** — não 16.
> - **Turbo deferred** — yarn workspaces + Makefile resolvem; reavaliar quando houver cache TS relevante.
> - **Jaeger e Karapace deferred** — entram na Fase 6 (observabilidade) e Fase 1 (contratos), respectivamente.

- [x] `package.json` raiz (yarn 4 workspaces `apps/*` + `libs/*`), `.yarnrc.yml`, `.editorconfig`, `.prettierrc`, `.prettierignore`, `.gitignore` consolidado.
- [x] `go.work` na raiz (`use ./apps/api`).
- [x] `infra/docker/docker-compose.yml` com Postgres 18.3 + Apache Kafka (KRaft). Jaeger e Karapace deferred.
- [x] **Makefile raiz** (`bootstrap`, `up/down/restart/nuke/ps/logs`, fan-out `dev/test/lint/build/clean`, padrão `make app-<nome> CMD=<target>`) + **`infra/docker/Makefile`** (`up/down/restart/nuke/ps/logs/pull/psql/pg-dump/kafka-topics/kafka-shell/kafka-create-topic`).
- [x] `lefthook.yml` (pre-commit: gofumpt + go vet + prettier + gitleaks; commit-msg: Conventional Commits). Targets `make install-hooks` / `make uninstall-hooks`.
- [x] CI inicial em GitHub Actions (`.github/workflows/ci.yml`): jobs `api` (vet+test+build+golangci-lint), `node-deps` (yarn install --immutable), `gitleaks`. Filtro `paths-filter` por app (api/web/anti-fraud) pra evitar rodar tudo a cada PR. Per-app Node CI entra junto com cada scaffold.
- [x] Esqueleto `apps/api` (Go 1.24 + chi v5 + slog, layout `cmd/api` + `internal/{config,httpapi}`, `/healthz` e `/readyz` com testes, Dockerfile distroless multi-stage, `Makefile` próprio).
- [x] Esqueleto `apps/web` (Next 16.2.6 + React 19, App Router, TS estrito, ESLint flat config + `next/typescript`, vitest, `Makefile` próprio). Build + lint + typecheck verdes.
- [x] Esqueleto `apps/anti-fraud` (Nest 11.1.19 + RxJS, kafkajs já em deps p/ Fase 4, healthz/readyz controller com testes Jest, ESLint flat config TS, `Makefile` próprio). `make test/build/lint` verdes.

**Critério de pronto:** `make bootstrap` em máquina nova instala deps + sobe stack saudável; `make test` passa zerado.

**Status Fase 0:** ✅ **Concluída.** `make test` da raiz roda fan-out triplo (Go via apps/api, Vitest via apps/web, Jest via apps/anti-fraud). Próxima parada: Fase 1 — contratos (OpenAPI + AsyncAPI + codegen).

---

## Fase 1 — Contratos primeiro (semana 2)

**Objetivo:** acabar com o drift que existe hoje entre os 3 repos legados.

> **Deltas vs. plano original:**
> - ADR foi numerada **0003** (não 0001) porque já existem `0001-ddd-hexagonal-by-context.md` e `0002-outbox-inbox-for-kafka-atomicity.md`.
> - Codegen TS usa **`openapi-typescript`** (drwpow), não `openapi-ts` — gera tipos puros, sem cliente HTTP, alinhado com "contratos = types".
> - oapi-codegen registrado como **tool no `go.mod`** de `libs/contracts/generated/go` (Go 1.24 `tool` directive), não via `go install` global.
> - Eventos: AsyncAPI + JSON Schema da spec **`identity.merchant.created.v1`** comitados; **codegen de eventos diferido** (entra quando Identity/Payments precisarem publicar de fato).

- [x] `libs/contracts/http/` — OpenAPI 3.1 para Identity (real, com `POST /v1/merchants`), Payments e Billing (skeletons).
- [x] `libs/contracts/events/asyncapi.yaml` + JSON Schema 2020-12 (`schemas/identity/merchant-created.v1.json`).
- [x] Codegen Go (oapi-codegen v2.7) → `libs/contracts/generated/go/identity/types.gen.go`. TS (openapi-typescript 7) → `libs/contracts/generated/ts/identity.d.ts`. Re-export ergonômico via `@iris-gate/contracts` (workspace yarn).
- [x] Schema registry **Karapace latest** em `infra/docker/docker-compose.yml` (compatibility `BACKWARD`, depende do healthcheck do kafka).
- [x] **ADR-0003** "Contratos como Single Source of Truth" (`docs/adr/0003-contracts-as-source-of-truth.md`).
- [x] Targets `make gen-contracts` e `make gen-contracts-check` (gate anti-drift). CI: jobs `contracts`, `web`, `anti-fraud` adicionados ao workflow, com `paths-filter` cruzando contratos↔consumers.

**Critério de pronto:** ✅ verificado por smoke test — renomear `legal_name` no `identity.openapi.yaml` quebra simultaneamente o build Go (`apps/api`: campo `LegalName` desaparece da struct) e o typecheck TS (`apps/web`: `Pick<Merchant, 'legal_name'>` deixa de ser válido). Reverter restaura ambos.

**Status Fase 1:** ✅ **Concluída.** Próxima parada: Fase 2 — Identity (domain + use-cases + sqlc + handler real substituindo o stub).

---

## Fase 2 — Identity (semana 3)

**Objetivo:** merchants conseguem criar conta e gerar API keys com segurança real.

> **Deltas vs. plano original:**
> - **Operators e Role** ficam pra Fase 7 (hardening); `apps/web` ainda não tem painel — sem ator humano além do dono do tenant.
> - **RotateApiKey** diferida (`IssueApiKey` + `RevokeApiKey` cobrem o caso até a Fase 5/web ter UX que peça rotação).
> - **sqlc** preterido em favor de **pgx v5 hand-rolled** nesta fase (poucas queries, 2 tabelas; tooling sqlc/Docker traz custo desproporcional). Migração para sqlc fica documentada como follow-up quando o volume de queries justificar.
> - **Migrations**: SQL embarcado via `go:embed` em `libs/identity/infra/pgstore/migrations/` aplicado via `MigrateEmbedded()` no boot (gate `MIGRATE_ON_STARTUP`). Atlas/migrate como tool oficial diferido.
> - **Testcontainers integration test** diferido — testes de handler com `httptest` + fakes já provam o critério; integração contra Postgres real é follow-up de baixo risco (pgstore é hand-rolled e simples).
> - **Token format**: `irk_<prefix>.<secret>` (`irk` = iris-gate key). Prefix 16 hex chars (8 bytes), secret 32 base64url chars (24 bytes), Argon2id default OWASP 2024 (m=64MB, t=1, p=4).
> - **Go bumpado pra 1.25** (forçado por `golang.org/x/crypto` v0.51 que requer 1.25). `toolchain go1.25.10` no go.work + go.mods.

- [x] `libs/identity/domain/` — Merchant + APIKey (com factory que valida CNPJ/email/range), erros sentinela, parser/formatter `irk_*.*`. Testes unitários verdes (10+ casos).
- [x] `libs/identity/application/` — ports (`MerchantRepo`, `APIKeyRepo`, `Hasher`, `SecretGen`, `Clock`, `IDGen`) + use cases `CreateMerchant`, `IssueAPIKey`, `AuthenticateAPIKey`, `RevokeAPIKey`. Testes com fakes in-memory cobrem fluxo full (criar → emitir → autenticar → revogar → 403).
- [x] `libs/identity/infra/pgstore/` — adapter pgx v5 implementando os ports; `go:embed` + `MigrateEmbedded()` aplicando `migrations/0001_init.sql` (tabelas `merchants` e `api_keys`).
- [x] `libs/identity/infra/hasher/` — Argon2id (PHC string format) + `SecretGenerator` (crypto/rand). Tests roundtrip + uniqueness.
- [x] `libs/identity/infra/httpapi/` — handlers chi `POST /v1/merchants`, `POST /v1/api-keys`, `POST /v1/api-keys/{id}/revoke`, `GET /v1/me` + middleware **`RequireAPIKey`**. Erros mapeados para `application/problem+json`.
- [x] `apps/api/cmd/api/main.go` reescrito como composition root (pgxpool + Argon2 + use cases + chi router). Migração roda automaticamente no boot quando `MIGRATE_ON_STARTUP=true`.
- [x] OpenAPI estendido (`libs/contracts/http/identity.openapi.yaml`) com endpoints + `securitySchemes.bearerAuth`. Tipos Go/TS regenerados.

**Critério de pronto:** ✅ verificado por testes `httptest` em `libs/identity/infra/httpapi/handlers_test.go`:
- request com API key válida → **200** (`TestAuth_ValidKey_Returns200`)
- request sem header / token malformado / secret errado / prefix inexistente → **401** (`TestAuth_InvalidKey_Returns401` com 5 sub-cases)
- request com chave revogada (via use case ou via endpoint HTTP) → **403** (`TestAuth_RevokedKey_Returns403` + `_Via_HTTP_Endpoint_*`)
- responses em formato RFC 7807 (`application/problem+json`) com `status` e `title`
- `slog` JSON em todos os caminhos (request id propagado pelo middleware chi).

**Status Fase 2:** ✅ **Concluída.** Próxima: Fase 3 — Payments (PaymentIntent, Outbox, idempotência, sandbox de adquirente).

---

## Fase 3 — Payments (semanas 4-5)

**Objetivo:** criar PaymentIntent, autorizar via 1 adquirente real (sandbox), publicar evento.

> **Deltas vs. plano original (slice MVP):**
> - **Acquirer real (Stone) diferido** — `FakeAcquirer` no MVP (sempre approves). Real entra em PR subsequente quando houver credenciais sandbox.
> - **Outbox + relay worker**: tabela `outbox_messages` + `CreateWithOutbox` em transação ✅. Relay worker (push pra Kafka) **stub** — método `PullUnpublished()` pronto, loop de publish entra junto com o adquirente real.
> - **Idempotency middleware**: `libs/payments/infra/idempotency` (em vez de `libs/platform/idempotency`) — fica perto do contexto que usa primeiro; promove pra `libs/platform/` quando outro contexto pedir.
> - **AuthorizePayment / CapturePayment / Refund** diferidos — entram com o acquirer real (sem ele, autorizar é só mudar status).
> - **Web tokenization** diferida — entra na **Fase 5** (Web), que é quando há UI pra tokenizar.

- [x] `libs/payments/domain/` — `Money` (VO com validação ISO-4217), `PaymentIntent` (status enum + factory que valida), errors sentinela, evento `PaymentIntentCreatedV1`.
- [x] `libs/payments/application/` — ports (`PaymentIntentRepo` com `CreateWithOutbox`, `Acquirer`, `Clock`, `IDGen`) + use case `CreatePaymentIntent` (cria + serializa evento + escreve outbox em uma chamada). Authorize/Capture diferidos.
- [x] `libs/payments/infra/pgstore/` — adapter pgx v5; transação `CreateWithOutbox` (intent + outbox atômico); `IdempotencyStore` (Postgres-backed); `MigrateEmbedded` aplicando `0002_payments.sql` (tabelas `payment_intents`, `outbox_messages`, `idempotency_records`).
- [x] `libs/payments/infra/acquirer/fake.go` — `FakeAcquirer` (sempre approves) — placeholder até Stone sandbox.
- [x] `libs/payments/infra/idempotency/` — middleware HTTP (Stripe-style: lookup → execute → save; mesma key + body diferente = 409). Tests cobrem replay + conflict + missing header.
- [x] `libs/payments/infra/httpapi/` — handler `POST /v1/payment-intents` protegido por `RequireAPIKey` + idempotency middleware via composição. Mapping DTO ↔ domain.
- [x] OpenAPI estendido (`libs/contracts/http/payments.openapi.yaml`) com Money, CreatePaymentIntentRequest, PaymentIntent, header `Idempotency-Key`. Tipos Go/TS regenerados.
- [x] `apps/api/cmd/api/main.go` monta payments no composition root (pgxpool partilhado com Identity, idempotency middleware, fake acquirer).

**Critério de pronto:** ✅ **parcialmente verificado** (slice MVP):
- Idempotência funciona end-to-end: 2 requests com mesma `Idempotency-Key` + body idêntico → mesma resposta, handler executa 1×, header `Idempotent-Replay: true` na 2ª. Verificado em `middleware_test.go`.
- Body diferente com mesma key → 409.
- Outbox atômico: criar intent escreve linha em `outbox_messages` na mesma tx (verificado por unit test com fake repo).
- Auth obrigatório: requests sem `Authorization: Bearer ...` → 401 (já provado pela Fase 2).

**Pendente para fechar critério integral** (web tokeniza → backend cria → publica → captura no painel):
- Stone sandbox adapter substituindo `FakeAcquirer`.
- Relay worker Kafka (loop `PullUnpublished` + publish + mark `published_at`).
- Authorize/Capture endpoints.
- Web tokenization na Fase 5.

**Status Fase 3:** 🟡 **Slice MVP concluída.** Próxima parada: continuar com Fase 4 (Anti-fraud) em paralelo, ou completar Stone + relay worker. Decisão fica com o time.

---

## Fase 4 — Anti-fraud (semana 6, paralelo à Fase 3)

**Objetivo:** decisão de fraude assíncrona com Inbox + Specification Pattern modernizado.

- [ ] `apps/anti-fraud/` — Nest 11.1 com `@nestjs/microservices` (Kafka transport) ou KafkaServer custom (mantém padrão do legado).
- [ ] `libs/fraud/domain/` — RiskAssessment, Decision (Approve/Review/Reject), Score.
- [ ] `libs/fraud/application/` — AssessRisk use-case + RuleRegistry (carrega de DB).
- [ ] `libs/fraud/infra/` — repos Prisma, consumer Kafka com **Inbox**, producer com **Outbox**.
- [ ] Migrar as 3 specifications do legado (suspicious-account, unusual-amount, frequent-high-value) com **score numérico** e **weights**.
- [ ] DLQ topic `fraud.assessment.failed.dlq`.
- [ ] Testes: cada Specification isolada + use-case com fakes.

**Critério de pronto:** mensagem em `fraud.assessment.requested.v1` é processada exatamente uma vez (mesmo com redelivery), responde em `fraud.assessment.completed.v1`, ou vai pra DLQ com motivo.

---

## Fase 5 — Web (semana 7)

**Objetivo:** painel funcional com auth segura e tipos gerados.

- [ ] `apps/web/` — Next 16 App Router, copia componentes shadcn úteis do `gateway-web` legado.
- [ ] Auth: NextAuth/Auth.js v5 com Iron-Session. API key do merchant fica no Vault, server-only.
- [ ] `libs/sdk-web/` — wrapper tipado em cima de `libs/contracts/generated/ts/`.
- [ ] Telas MVP: login, dashboard, criar invoice, listar payment intents, detalhe da intent.
- [ ] React Query (TanStack) para state server. Suspense + error boundaries.
- [ ] Testes: Playwright para 3 fluxos críticos.

**Critério de pronto:** Lighthouse > 90 em performance/best-practices; nenhuma API key visível em devtools.

---

## Fase 6 — Observabilidade end-to-end (semana 8)

- [ ] OTel SDK em todos os serviços.
- [ ] Trace context propagado em HTTP + Kafka headers.
- [ ] Dashboards Grafana: latência por endpoint, taxa de aprovação por adquirente, lag dos consumers.
- [ ] Alertas Prometheus: SLO de latência (p95 < 300ms), error budget mensal.
- [ ] Audit log append-only para decisões de fraude e mudanças de API key.

---

## Fase 7 — Hardening de segurança (semana 9)

- [ ] CSP estrito + restante dos headers de §11 do `security.md`.
- [ ] Rate limit por API key (Redis sliding window).
- [ ] Row-level security no Postgres.
- [ ] `gitleaks` + `govulncheck` + `pnpm audit` bloqueantes em CI.
- [ ] Threat model review com pessoa externa.
- [ ] Pen-test interno (mínimo: OWASP ZAP automatizado).

---

## Fase 8 — Webhooks + Notifications (semana 10)

- [ ] `libs/notifications/` — Webhook, DeliveryAttempt.
- [ ] HMAC signing.
- [ ] Retry com exponential backoff (1m, 5m, 30m, 2h, 6h, 24h).
- [ ] Dashboard de deliveries no painel.

---

## Fase 9 — Ledger mínimo (semana 11)

- [ ] `libs/ledger/` — Account, JournalEntry append-only.
- [ ] Eventos `ledger.posting.*` consumidos de Payments.
- [ ] Endpoint de saldo + extrato.

---

## Fase 10 — Beta privado (semana 12)

- [ ] 3 merchants pilotos com sandbox real.
- [ ] Runbooks: incidente, rotação de chave, restore de DB.
- [ ] Status page pública.
- [ ] Documentação developer-facing (portal).

---

## Pós-MVP (não-bloqueante para v1)

- Vault próprio de tokenização (PCI-DSS Level 1)
- ML model serving para fraude
- Multi-região / DR ativo-ativo
- Saga orchestrator (Temporal)
- Open Finance / Pix Automático
- SDK oficial para Node, Python, Go
- Marketplace de plugins

---

## Métricas de sucesso do MVP

| Métrica                                    | Alvo                  |
| ------------------------------------------ | --------------------- |
| Latência p95 `POST /payment-intents`       | < 300ms               |
| Taxa de aprovação no fluxo síncrono        | > 95% (sandbox)       |
| Lag p95 do consumer fraud                  | < 500ms               |
| Cobertura de testes nos pacotes `domain/`  | > 90%                 |
| Zero PAN em logs (lint passa)              | 100%                  |
| Idempotency replay correto                 | 100% (teste de chaos) |
| Mean time to recovery (incidente simulado) | < 30 min              |
