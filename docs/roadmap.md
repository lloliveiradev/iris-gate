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

- [ ] `libs/identity/domain/` — Merchant, ApiKey (value object), Operator, Role.
- [ ] `libs/identity/application/use-cases/` — CreateMerchant, IssueApiKey, RotateApiKey, RevokeApiKey, AuthenticateApiKey.
- [ ] `libs/identity/infra/persistence/` — Postgres (sqlc) com tabelas `merchants`, `api_keys` (Argon2id), `operators`.
- [ ] `apps/api/` — handlers chi expondo `POST /merchants`, `POST /api-keys`, `POST /api-keys/rotate`.
- [ ] Middleware `RequireApiKey` com lookup por prefix + Argon2 verify.
- [ ] Testes: unit no domínio, integração no use-case (testcontainers Postgres).

**Critério de pronto:** request com API key válida passa, com inválida retorna 401, com revogada retorna 403, todos com trace.

---

## Fase 3 — Payments (semanas 4-5)

**Objetivo:** criar PaymentIntent, autorizar via 1 adquirente real (sandbox), publicar evento.

- [ ] `libs/payments/domain/` — PaymentIntent, Money, CardToken, IdempotencyKey, eventos.
- [ ] `libs/payments/application/` — CreatePaymentIntent, AuthorizePayment, CapturePayment.
- [ ] `libs/payments/infra/acquirers/stone/` (ou outro) — adapter para sandbox.
- [ ] `libs/platform/outbox/` — implementação Outbox + relay worker.
- [ ] `libs/platform/idempotency/` — middleware HTTP.
- [ ] `apps/api/` — rotas `POST /payment-intents`, `POST /payment-intents/{id}/capture`.
- [ ] Tokenização: integração com SDK do adquirente em `apps/web` (PAN nunca chega no backend).

**Critério de pronto:** fluxo end-to-end no sandbox: web tokeniza → backend cria intent → publica evento → captura aparece no painel.

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
