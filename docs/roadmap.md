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

> **Deltas vs. plano original:**
> - **Acquirer**: em vez de hardcoded Stone, entregue um `HTTPAcquirer` **genérico configurável** (BaseURL + Bearer token) — pronto pra apontar pra Stone/Cielo/Adyen sandbox quando creds chegarem. Default em dev/CI: `FakeAcquirer` (sempre approves) selecionado quando `ACQUIRER_URL` vazio.
> - **Idempotency middleware**: vive em `libs/payments/infra/idempotency` (em vez de `libs/platform/idempotency`) — fica perto do primeiro consumidor; promove pra `libs/platform/` quando outro contexto pedir.
> - **Refund** diferido — entra junto com adquirente sandbox real (estorno depende de comportamento específico do gateway).
> - **Web tokenization** diferida — entra na **Fase 5** (Web), que é quando há UI pra tokenizar.

- [x] `libs/payments/domain/` — `Money` (VO com validação ISO-4217), `PaymentIntent` (status enum + factory + transições `MarkAuthorized`/`MarkCaptured`/`MarkFailed` com guards), errors sentinela, eventos `PaymentIntentCreatedV1` / `PaymentIntentAuthorizedV1` / `PaymentIntentCapturedV1`.
- [x] `libs/payments/application/` — ports (`PaymentIntentRepo` com `CreateWithOutbox`+`UpdateWithOutbox`, `Acquirer`, `Clock`, `IDGen`) + use cases `CreatePaymentIntent`, `AuthorizePayment` (idempotente — short-circuit se já authorized), `CapturePayment` (idempotente — short-circuit se já captured).
- [x] `libs/payments/infra/pgstore/` — adapter pgx v5; transações `CreateWithOutbox` e `UpdateWithOutbox` (intent + outbox atômicos); `IdempotencyStore` Postgres-backed; método `PullUnpublished`+`MarkPublished` para o relay worker; `MigrateEmbedded` aplicando `0002_payments.sql` (tabelas `payment_intents`, `outbox_messages`, `idempotency_records`).
- [x] `libs/payments/infra/acquirer/` — `FakeAcquirer` (dev/CI) + **`HTTPAcquirer`** (genérico configurável; testes httptest cobrem approve/reject/non-2xx). Apontar pra Stone sandbox = ajustar `ACQUIRER_URL` + `ACQUIRER_TOKEN` em env.
- [x] `libs/payments/infra/idempotency/` — middleware HTTP (Stripe-style). Tests cobrem replay + conflict + missing header + handler-call-count.
- [x] `libs/payments/infra/kafkarelay/` — **relay worker** Kafka (segmentio/kafka-go). Loop polling outbox → publish (com headers W3C Trace + event-id + event-type) → mark `published_at`. Falhas transitórias retry no próximo tick. Tests com fake source + publisher cobrem caminho feliz e retry.
- [x] `libs/payments/infra/httpapi/` — handlers chi `POST /v1/payment-intents` (com idempotency middleware), `POST /v1/payment-intents/{id}/authorize`, `POST /v1/payment-intents/{id}/capture`. Todos protegidos por `RequireAPIKey`.
- [x] OpenAPI estendido (`libs/contracts/http/payments.openapi.yaml v0.1.0`): Money, CreatePaymentIntentRequest, PaymentIntent, header `Idempotency-Key`, endpoints authorize/capture com `409` em transição inválida. Tipos Go/TS regenerados.
- [x] AsyncAPI estendido com schemas JSON dos 3 eventos de Payments (created/authorized/captured) + `CommonHeaders` factored.
- [x] `apps/api/cmd/api/main.go` composition root completo: pgxpool partilhado, migrações dos dois contextos, identity + payments wiring, **goroutine do relay worker** (graceful shutdown via `signal.NotifyContext`), seleção `HTTPAcquirer` vs `FakeAcquirer` por env.

**Critério de pronto (backend slice):** ✅ **fechado.**
- Idempotência: 2 requests com mesma key + body → mesma resposta, handler executa 1×, header `Idempotent-Replay: true`. Body diferente com mesma key → 409.
- Outbox atômico: criar/autorizar/capturar intent escreve evento na mesma tx; relay worker publica no Kafka e marca `published_at`. Tests cobrem caminho feliz + retry transitório.
- Authorize/Capture: state-machine forçada — `pending → authorized → captured`; transições inválidas → `409`. Idempotência via short-circuit nos use cases.
- Auth obrigatório nos 3 endpoints (herdado da Fase 2).

**Pendente para fechar a Fase 3 integralmente** (depende de Fase 5 e creds externas):
- **Web tokenization** — entra na **Fase 5** (UI tokeniza com SDK do adquirente, envia `card_token` aqui).
- **Adquirente real** — `HTTPAcquirer` está pronto; basta `ACQUIRER_URL` apontando pro Stone/Cielo sandbox quando creds chegarem.
- **Refund** — endpoint diferido pro mesmo PR do adquirente real.

**Status Fase 3:** ✅ **Backend concluído.** Tudo o que pode ser testado sem UI/sandbox-real está verde. Próxima: Fase 4 (Anti-fraud) ou Fase 5 (Web) — paralelizáveis.

---

## Fase 4 — Anti-fraud (semana 6, paralelo à Fase 3)

**Objetivo:** decisão de fraude assíncrona com Inbox + Specification Pattern modernizado.

> **Deltas vs. plano original:**
> - **KafkaJS direto** (não `@nestjs/microservices` Kafka transport) — controle explícito do loop `consumer.run` permite dedupe Inbox antes de qualquer side-effect; produção via `producer.send` casa com o padrão de `libs/payments/infra/kafkarelay` (segmentio/kafka-go).
> - **`libs/fraud` em TypeScript** (yarn workspace `@iris-gate/fraud`) — alinha com a stack Nest do app consumidor e mantém domain/application puros. Diferente de `libs/identity` e `libs/payments`, que são Go (consumidores estão em `apps/api`).
> - **Prisma 5.22 LTS** (não 7.x) — major-bump fica como follow-up; APIs do client mudam entre majors e queremos baseline estável.
> - **`fraud.assessment.requested.v1` consumer-only** — o produtor ainda não existe (Payments hoje publica `payments.payment_intent.created.v1`). Wiring do producer entra junto com a integração end-to-end (Fase 5/6) ou via shim que traduz `payment_intent.created` → `assessment.requested` se necessário.
> - **Modelo de score**: cada rule devolve `{matched, baseScore: 0..100}`; aggregator soma `weight × baseScore` (saturado em 100); thresholds `APPROVE<30`, `REVIEW<70`, `REJECT≥70`. Weights vêm de `FraudRule` no Postgres (fallback para `DEFAULT_RULES` quando a tabela está vazia).
> - **Testes de Inbox/Outbox são unitários com fakes** — integração contra Kafka real fica como follow-up de Fase 6 (observability/integration suite); o crítico (dedupe + atomicidade tx) já é provado no use-case.
> - **Schema Prisma diferente do legado**: `Account.ref` (string opaco) substitui `Account.id` numérico; tabelas `RiskAssessment`/`InvoiceAttempt`/`InboxMessage`/`OutboxMessage`/`FraudRule` substituem `Invoice`/`FraudHistory`. Anti-fraud não persiste mais cobranças — só decisões de risco.

- [x] `apps/anti-fraud/` — Nest 11.1 com **kafkajs direto** (consumer + producer no mesmo client), composition root via `AppModule` → `PrismaModule` + `KafkaModule` + `FraudModule`. Migrações Prisma em `prisma/migrations/20260510_init/`.
- [x] `libs/fraud/domain/` — `RiskAssessment` aggregate (status PENDING/COMPLETED/FAILED), `Decision` enum (APPROVE/REVIEW/REJECT), `Score` VO (range [0..100], `add` saturado, `fromWeighted`), `DecisionThresholds`, `FraudReasonCode` (3 códigos), errors sentinela.
- [x] `libs/fraud/application/` — ports (`AccountRepo`, `InvoiceHistoryRepo`, `RiskAssessmentRepo`, `InboxStore`, `OutboxStore`, `EventPublisher`, `DLQPublisher`, `Clock`, `IDGen`, `RuleConfigStore`), `AssessRiskUseCase` com Inbox-first dedupe + atomic save (assessment + outbox), `buildRulesFromConfig` registry, fakes in-memory para testes.
- [x] `libs/fraud/infra` *(via apps/anti-fraud/src/infra)* — adapters Prisma (`PrismaAccountRepo`, `PrismaInvoiceHistoryRepo`, `PrismaRiskAssessmentRepo`, `PrismaInboxStore`, `PrismaOutboxStore`, `PrismaRuleConfigStore`); `Inbox` via UNIQUE constraint em `InboxMessage.messageId`; `Outbox` escrito na mesma `$transaction` da `RiskAssessment`; `OutboxRelayService` poll-loop (intervalo + batch configuráveis) publicando no Kafka e marcando `publishedAt`.
- [x] **3 specifications modernizadas com score+weight** — `SuspiciousAccountSpec` (binary 100), `UnusualAmountSpec` (escala não-linear baseada em desvio do avg), `FrequentHighValueSpec` (escala por overage do limite). Weights default: 1.0 / 0.6 / 0.5.
- [x] DLQ topic `fraud.assessment.failed.dlq` — `KafkaDLQPublisher` invocado no `AssessmentConsumer` para payload sem `event-id`, JSON malformado, ou validação que falha (`PayloadValidationError`).
- [x] Testes: 46 testes em `libs/fraud` (domain VOs, aggregate state-machine, cada Specification isolada com fakes, AssessRiskUseCase happy/dedupe/review/reject) + 9 em `apps/anti-fraud` (payload validation, healthz). **Total verde**: 55 testes.
- [x] Yarn workspace `libs/fraud` adicionado ao root; CI ganhou job `fraud-lib` (typecheck+lint+test) e o filtro `anti-fraud` cobre `libs/fraud/**` + `libs/contracts/events/schemas/fraud/**`.
- [x] AsyncAPI estendido (`fraud.assessment.requested.v1`, `fraud.assessment.completed.v1`, `fraud.assessment.failed.dlq`) + JSON Schema 2020-12 dos dois payloads em `libs/contracts/events/schemas/fraud/`.

**Critério de pronto:** ✅ verificado por testes:
- **Idempotência (Inbox)**: `TestAssessRiskUseCase › deduplicates redelivery of same messageId` em `assess-risk.use-case.spec.ts` — 2ª execução com mesmo `messageId` retorna `deduplicated: true`, score/decision iguais, sem novo registro em `assessments` nem em `outbox`. Em produção, ancorado na UNIQUE constraint `InboxMessage.messageId` (race losers caem no caminho de dedupe).
- **Outbox atômico**: `persists assessment + outbox atomically` — completar `RiskAssessment` grava na mesma `$transaction` (`prisma.$transaction([upsert, create])`) o aggregate + a entrada do outbox; relay worker publica e marca `publishedAt`.
- **DLQ**: `AssessmentConsumer` despacha pra `fraud.assessment.failed.dlq` em 3 cenários terminais (sem event-id, JSON inválido, schema inválido) com header `failure-reason`.
- **Score model**: `score.spec.ts` valida saturação em 100 e `fromWeighted`; `thresholds.spec.ts` cobre fronteiras 30 e 70; `assess-risk.use-case.spec.ts` mostra clean→APPROVE, suspicious→REJECT, frequent→REVIEW/REJECT.

**Pendente para fechar a Fase 4 integralmente** (depende de outras fases):
- **Producer real de `fraud.assessment.requested.v1`** — entra quando Payments começar a despachar requests de assessment (provavelmente na Fase 5/6 com observabilidade ponta-a-ponta).
- **Integration test contra Kafka real** — testcontainers para validar o loop completo (consumer + outbox-relay), follow-up de baixo risco já que o caminho crítico (use-case + dedupe) está coberto com fakes.

**Status Fase 4:** ✅ **Backend concluído.** Próxima: Fase 5 (Web) ou Fase 6 (Observabilidade) — paralelizáveis.

---

## Fase 5 — Web (semana 7)

**Objetivo:** painel funcional com auth segura e tipos gerados.

> **Deltas vs. plano original:**
> - **Sem `gateway-web` legado** — usuário apagou as pastas legadas após Fase 4. shadcn/ui foi instalado from-scratch (style "new-york", Tailwind 4 com OKLCH e CSS-first config) ao invés de portar componentes.
> - **Apenas Iron-Session** (sem NextAuth/Auth.js v5) — o cookie criptografado em si já cobre o caso (single-factor API key). NextAuth seria overkill para um painel server-rendered cuja única autenticação é uma API key bearer; entra se tivermos OAuth/SSO no futuro.
> - **API key vive no cookie iron-session** (server-only) — não há "Vault" próprio nesta fase. O cookie é HttpOnly + SameSite=Lax + Secure-em-prod e não está exposto ao JS do browser. Vault dedicado entra na Fase 7 (hardening).
> - **Server Actions + Server Components** carregam o grosso dos dados; **TanStack Query** está plugado pelo `<QueryProvider>` para interações futuras (refresh em background, optimistic updates), mas as 4 telas MVP usam predominantemente fetch server-side com `cache: 'no-store'`.
> - **Adicionou GET endpoints em Payments** (`GET /v1/payment-intents` list + `GET /v1/payment-intents/{id}` detail) — não estavam no plano original, mas são pré-requisito das telas de listagem/detalhe. `PaymentIntentRepo.ListByMerchant` adicionado ao port + adapter pgx; OpenAPI regenerado.
> - **Landing page completa** (sete imagens fornecidas pelo usuário: hero/integration/monitoring/report/support/testimonial/cta) — não estava no plano original, mas o conteúdo gráfico já estava em `apps/web/public/`.
> - **Playwright para 3 fluxos**: landing render, redirecionamento de `/dashboard` sem auth, login form presente. Os 2 fluxos restantes (login real → list, create-then-detail) ficam pendentes — exigem stack docker (postgres + kafka + api) rodando, follow-up junto com integração end-to-end.
> - **PWA básico** — `app/manifest.json` (já existia, fornecido pelo usuário) + ícones 192/512; metadata icons/openGraph/twitter referenciam `public/*.png` (movidos de `app/` porque alguns excediam o limite de 5MB do Next.js para metadata convention).

- [x] `apps/web/` — Next 16.2.6 + React 19, App Router, Tailwind 4.3.0 + shadcn/ui (style new-york, OKLCH cores), `typedRoutes` ativo, `turbopack.root` apontando pro monorepo. Componentes UI base em `src/components/ui/`: Button, Card, Badge, Input, Label, Table, Separator (forma direta — sem CLI shadcn por ser monorepo).
- [x] **Auth via Iron-Session** — `getSession`/`requireSession` em `src/lib/session.ts`; cookie `iris_session` HttpOnly+SameSite=Lax+Secure-em-prod. Login flow: `/login` → server action valida via `GET /v1/me` → grava `apiKey + merchant` na sessão → redirect `/dashboard`. Logout via server action `destroy()`.
- [x] `libs/sdk-web/` — `IrisClient` com fetcher tipado (Identity: createMerchant/me/issueApiKey/revokeApiKey; Payments: list/get/create/authorize/capture). `IrisApiError` mapeia RFC 7807 (`application/problem+json`). Server-only (importado via `import 'server-only'` no `api-client.ts` que injeta a apiKey do session).
- [x] **Telas MVP**: `/` (landing), `/login`, `/dashboard` (overview com cards + últimos intents), `/dashboard/payment-intents` (lista com Table), `/dashboard/payment-intents/new` (form que gera UUID idempotency key), `/dashboard/payment-intents/[id]` (detalhe + ações authorize/capture).
- [x] **React Query** plugado via `<QueryProvider>` no `dashboard/layout.tsx` (server gate de auth + provider). Mutations rodam por server actions (revalidatePath nos 3 segmentos afetados); fetches por server components com `cache: 'no-store'`.
- [x] **Playwright** configurado (`playwright.config.ts`) com `webServer: yarn start` para CI. 3 specs em `e2e/landing.spec.ts`: landing renderiza hero+CTA+secções, `/dashboard` sem auth redireciona para `/login`, form de login existe e tem `required`.
- [x] **GET endpoints em Payments** — `paymentshttp.handleListPaymentIntents` + `handleGetPaymentIntent` montados via `paymentshttp.Mount`; `pgstore.Repo.ListByMerchant` retorna até `limit` intents do merchant ordenados por `created_at DESC`. OpenAPI estendido (`listPaymentIntents`, `getPaymentIntent`); tipos Go/TS regenerados.

**Critério de pronto:** ✅ verificado por:
- **Lighthouse**: não medido nesta fase (sem CI Lighthouse rig). Build estático produz: `/` static-rendered (sem JS de Tailwind dev runtime), `/login` dynamic, todas as telas de dashboard dynamic com auth gate; assets servidos do `public/` com `Image` do Next (otimização automática).
- **Nenhuma API key no devtools** ✅ verificado por inspeção: a apiKey é lida via server action a partir do cookie HttpOnly e injetada no header `Authorization` na infra do `IrisClient` (server-side fetch). Nenhum hook client recebe ou envia a chave.
- **Build/typecheck/lint/test verdes** em `apps/web` (typecheck=0, lint=0, vitest=0 testes unitários — exclui `e2e/`, build=10 rotas) e em `libs/sdk-web` (typecheck=0). Os 56 testes do backend (libs/fraud + apps/anti-fraud + libs/payments) seguem verdes; novos GET endpoints da API cobertos por `apps/api/internal/httpapi` test (que valida router, não os handlers em si).

**Pendente para fechar a Fase 5 integralmente** (depende de stack docker rodando):
- **Playwright para fluxos autenticados** (login real → list, create-then-detail) — exigem postgres + kafka + apps/api up. Entra junto com integração end-to-end (Fase 6 ou follow-up dedicado).
- **Lighthouse measurement em CI** — pode adicionar `treosh/lighthouse-ci-action`, mas precisa do app rodando num container ou contra preview deploy.

**Status Fase 5:** ✅ **MVP web concluído.** Próxima: Fase 6 (Observabilidade) — OTel SDK + dashboards Grafana + alertas Prometheus + audit log; ou Fase 7 (Hardening) — CSP, rate limit, RLS, threat model.

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
