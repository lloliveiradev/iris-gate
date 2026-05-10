# Arquitetura Alvo — IrisGate

> DDD estratégico + Hexagonal/Clean por bounded context. Comunicação síncrona via REST, assíncrona via Kafka com Outbox/Inbox. Contratos versionados como artefato de primeira classe.

---

## 1. Visão macro

```
                              ┌──────────────────────┐
                              │     apps/web         │
                              │   Next.js 15 (RSC)   │
                              │  shadcn · RHF · Zod  │
                              └──────────┬───────────┘
                                         │ HTTPS (BFF)
                                         ▼
                              ┌──────────────────────┐
                              │  apps/api    │
                              │   Go · chi · sqlc    │  ◀── inbound REST
                              │   Hexagonal · DDD    │      OAuth2/JWT + idempotency
                              └──────┬───────────┬───┘
                                     │           │
                            REST     │           │ Kafka (events)
                                     ▼           ▼
                       ┌──────────────────┐  ┌──────────────────┐
                       │ provedores       │  │  apps/anti-fraud │
                       │ adquirentes      │  │  NestJS · Prisma │
                       │ (Stone, Cielo,   │  │  Specification   │
                       │  Pix, boletos)   │  │  KafkaJS + DLQ   │
                       └──────────────────┘  └──────────────────┘

                              ┌──────────────────────┐
                              │      Postgres        │
                              │  + outbox + inbox    │
                              └──────────────────────┘

                              ┌──────────────────────┐
                              │  Kafka (Redpanda em  │
                              │   dev) + schema reg. │
                              └──────────────────────┘
```

---

## 2. Bounded contexts (DDD estratégico)

| Contexto | App owner | Linguagem ubíqua | Responsabilidade |
|----------|-----------|------------------|------------------|
| **Identity** | api | Merchant, ApiKey, Operator, Role | Cadastro de merchants, emissão e rotação de API keys, RBAC interno do painel. |
| **Billing** | api | Invoice, Customer, BillingCycle | Faturas, ciclos, cobrança recorrente. |
| **Payments** | api | PaymentIntent, PaymentMethod, Authorization, Capture, Refund | Orquestra a transação, decide rota (Pix, cartão, boleto, wallet), fala com adquirente. |
| **Fraud** | anti-fraud | RiskAssessment, Rule, Score, Decision | Avalia transação, devolve `Approve | Review | Reject` com score e motivo. |
| **Ledger** | api (futuro: serviço próprio) | Account, JournalEntry, Posting | Saldo dos merchants, conciliação, chargebacks. **Append-only**. |
| **Notifications** | api | Webhook, DeliveryAttempt | Webhooks assinados (HMAC) para merchants, retry com backoff exponencial. |

Mapa de contextos (Context Map):

```
   Identity ──(Customer/Supplier)──▶ Payments
                                       │
                                       │ Conformist
                                       ▼
                                     Billing
                                       │
                       ┌───────────────┼─────────────────┐
                       ▼               ▼                 ▼
                    Fraud           Ledger          Notifications
                  (ACL via         (Pub/Sub          (Pub/Sub
                   eventos)         eventos)          eventos)
```

- **Customer/Supplier**: Identity é supplier de Payments (Payments depende do contrato de Merchant/ApiKey).
- **ACL (Anti-Corruption Layer)**: Payments → Fraud só conversa via eventos publicados em tópicos versionados. Mudança de schema interno do Fraud não vaza para Payments.
- **Pub/Sub**: Ledger e Notifications são consumidores eventualmente consistentes do que Payments publica.

---

## 3. Layout do monorepo

Estado atual (esqueleto incompleto):
```
apps/api/                  ✓ existe (vazio)
libs/identity/{domain,application,infra}/   ✓ existe
libs/billing/{domain,application,infra}/    ✓ existe
libs/shared/                                ✓ existe
```

Layout alvo:
```
iris-gate/
├── apps/
│   ├── web/                       Next.js 15 (BFF + UI)
│   ├── api/                       Go (Identity, Billing, Payments, Ledger, Notifications)
│   └── anti-fraud/                NestJS (Fraud)
├── libs/
│   ├── contracts/                 ⭐ OpenAPI 3.1 + AsyncAPI 2.6 + JSON Schemas
│   │   ├── http/                  openapi.yaml por contexto
│   │   ├── events/                asyncapi.yaml + .avsc/.proto por evento
│   │   └── generated/             Go (oapi-codegen, kafka-go-gen) · TS (openapi-ts) · Nest (nestjs-zod)
│   ├── identity/                  domain/application/infra (Go-only)
│   ├── billing/                   domain/application/infra (Go-only)
│   ├── payments/                  domain/application/infra (Go-only)
│   ├── ledger/                    domain/application/infra (Go-only)
│   ├── notifications/             domain/application/infra (Go-only)
│   ├── fraud/                     domain/application/infra (TS-only, consumido pelo apps/anti-fraud)
│   ├── platform/                  Go: telemetry, kafka, outbox, inbox, http-server, config
│   ├── platform-ts/               TS: kafka-consumer base, otel, error-mapping
│   └── sdk-web/                   TS: cliente tipado para apps/web (gerado de contracts/)
├── infra/
│   └── docker/                    docker-compose.yml (postgres, kafka [+ jaeger, karapace deferred])
├── tools/
│   ├── migrations/                atlas configs por contexto
│   └── scripts/                   gen-contracts, seed (bootstrap saiu — ver Makefile raiz)
├── configs/
│   ├── otel-collector.yaml
│   └── prometheus.yml
└── docs/                          esta pasta
```

**Por que `libs/contracts/` é prioridade #1:** elimina o drift que existe hoje entre `go-gateway`, `anti-fraud` e `gateway-web` (snake_case vs camelCase, enums diferentes, payloads divergentes). Sem isso, qualquer DDD vira teatro.

---

## 4. Arquitetura interna por contexto (Hexagonal)

Cada `libs/<context>/` segue exatamente este layout (consistente com o que já foi scaffoldado em `libs/identity` e `libs/billing`):

```
libs/payments/
├── domain/                    núcleo. Sem dependência de framework, DB ou rede.
│   ├── entities/              PaymentIntent, Authorization, Capture
│   ├── value-objects/         Money, Currency, CardToken, IdempotencyKey
│   ├── events/                PaymentAuthorizedEvent, PaymentCapturedEvent
│   ├── repositories/          interfaces (PaymentIntentRepository)
│   └── errors/                erros de domínio tipados (InsufficientFunds, ExpiredToken)
├── application/               casos de uso. Orquestra domain + ports.
│   ├── ports/                 interfaces inbound (commands) + outbound (acquirers, fraud-client)
│   ├── use-cases/             AuthorizePayment, CapturePayment, RefundPayment
│   └── policies/              roteamento por país/método/risco (sem if/else espalhado)
└── infra/
    ├── persistence/           PaymentIntentPgRepository (sqlc + pgx)
    ├── http/                  handlers chi → use-cases (adapter)
    ├── kafka/                 OutboxPublisher, FraudResultConsumer
    ├── acquirers/             adapters Pix, Cielo, Stone, etc.
    └── observability/         OTel spans, métricas Prom
```

**Regra de dependência (Clean Arch):**

```
infra ──▶ application ──▶ domain
   ▲           │
   └───────────┘  (infra implementa interfaces de application/domain)
```

`domain/` **não importa nada de fora do próprio pacote**. `application/` só importa `domain/`. `infra/` é a única camada que toca DB, HTTP, Kafka.

---

## 5. Padrões transversais (não-negociáveis)

### 5.1 Outbox Pattern — atomicidade DB ↔ Kafka

Resolve o problema crítico do `go-gateway` legado (Save → Publish não atômico).

```
┌─────────────────────────────────────────────────────────┐
│ BEGIN TRANSACTION                                       │
│   INSERT INTO payment_intents (...)                     │
│   INSERT INTO outbox_events (aggregate_id, type, payload│
│                              status='pending')          │
│ COMMIT                                                  │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
              ┌───────────────────────┐
              │  Outbox Relay         │  poll a cada 200ms
              │  (worker no api-gw)   │  ou via PG NOTIFY/LISTEN
              └──────────┬────────────┘
                         │ produz em Kafka
                         ▼
                ┌────────────────┐
                │  Kafka topic   │
                └────────────────┘
              após ack: marca outbox row como 'sent'
```

Implementação em `libs/platform/outbox/` (Go) e `libs/platform-ts/outbox/` (TS).

### 5.2 Inbox Pattern — idempotência de consumo

Resolve o problema crítico do `anti-fraud` legado (consumer não-idempotente).

```
on_message(msg):
  BEGIN
    INSERT INTO inbox (message_id, consumer, processed_at)
       VALUES (msg.id, 'fraud-consumer', now())
       ON CONFLICT DO NOTHING
       RETURNING xmax = 0 AS inserted
    if not inserted: COMMIT and return  -- já processado, descarta
    -- aplica side-effects
    publish_result()
  COMMIT
```

### 5.3 Idempotency Keys em mutações HTTP

Toda rota `POST/PUT/DELETE` aceita header `Idempotency-Key: <uuid>`. Tabela `idempotency_responses` guarda `(key, request_hash, response_status, response_body, ttl=24h)`. Replay devolve resposta cacheada.

### 5.4 Tokenização de PAN

PAN **nunca toca a aplicação principal**. Fluxo:

```
Web/Mobile  ──TLS──▶  Vault de tokenização  ──▶  retorna card_token (opaco)
   │                  (Iris Vault, isolado, PCI-DSS scope mínimo)
   │
   └──── envia card_token (não PAN) ──▶ apps/api
                                              │
                                              ▼
                                       apps/anti-fraud
                                       adquirente (detokeniza só na call out)
```

Para MVP, usar **provedor de tokenização do adquirente** (Stone, Cielo oferecem). Operar Vault próprio só com PCI-DSS Level 1 e auditoria.

### 5.5 Observabilidade desde o dia zero

- **OpenTelemetry** SDK em Go (`otelhttp`, `otelkafka`, `otelpgx`) e Nest (`@opentelemetry/auto-instrumentations-node`).
- **Trace context** propagado em headers HTTP e em headers de mensagem Kafka.
- **Métricas RED** (Rate, Errors, Duration) por endpoint e por consumer group.
- Backend em dev: Jaeger + Prometheus + Grafana via `infra/docker/`. Backend em prod: à escolha (Tempo, Datadog, Honeycomb).

### 5.6 Comunicação síncrona entre serviços

`api → anti-fraud` é **assíncrona via Kafka** por padrão. Para casos onde decisão síncrona é necessária (autorização em < 200ms), expor gRPC interno com mTLS e timeout agressivo + fallback para "REVIEW" se latência > SLO.

---

## 6. Eventos de domínio (catálogo inicial)

Definidos em `libs/contracts/events/` com AsyncAPI 2.6 + Avro/JSON Schema. Tópicos com sufixo de versão:

```
payments.intent.created.v1            { intent_id, merchant_id, amount, currency, payment_method }
payments.intent.authorized.v1         { intent_id, authorization_id, acquirer, latency_ms }
payments.intent.captured.v1           { intent_id, captured_amount, captured_at }
payments.intent.refunded.v1           { intent_id, refund_id, amount }
payments.intent.failed.v1             { intent_id, reason_code, acquirer_response }

fraud.assessment.requested.v1         { intent_id, features, idempotency_key }
fraud.assessment.completed.v1         { intent_id, decision, score, reasons[], rules_version }

ledger.posting.requested.v1           { intent_id, debit_account, credit_account, amount }
ledger.posting.confirmed.v1           { posting_id, balance_snapshot }

notifications.webhook.queued.v1       { webhook_id, merchant_id, event_id }
```

Convenções:
- **Tópico**: `<contexto>.<agregado>.<evento>.v<n>`
- **Chave da mensagem**: `aggregate_id` (garante ordem por agregado dentro de uma partição)
- **Headers obrigatórios**: `traceparent`, `idempotency-key`, `event-id`, `published-at`
- **Schema registry**: Confluent compatível (Karapace em dev) — breaking change exige nova versão `vN+1` e período de dual-publish.

---

## 7. Toolchain (a criar — nada existe hoje)

### Versões-alvo (atualizado 2026-05-09)

| Componente | Versão | Notas |
|-----------|--------|-------|
| Go | **1.24** | usado em `apps/api`, `go.work` |
| Node | **20.x LTS** | runtime de `apps/web` e `apps/anti-fraud` |
| Postgres | **18.3** | imagem `postgres:18.3-alpine` em dev |
| Apache Kafka | **latest (KRaft)** | single-node em dev, sem Zookeeper |
| Next.js | **16.2.6** | App Router, React 19 |
| React | **19.x** | |
| NestJS | **11.1.19** | usa `@nestjs/microservices` Kafka transport |
| TypeScript | **5.7.x** | strict em todos os pacotes TS |
| chi (Go) | **v5.2.x** | |

### Toolchain

| Necessidade | Escolha | Justificativa |
|-------------|---------|---------------|
| Workspace Go | `go.work` na raiz | suportado oficialmente desde Go 1.18 |
| Workspace TS | **Yarn 4 (Berry)** workspaces, `nodeLinker: node-modules` | escolhido em vez de pnpm; PnP costuma quebrar com Next/Nest |
| Orquestrador de tasks | **Makefile** raiz delegando + um por galho (`apps/*`, `infra/*`) | substitui Turborepo no dia 1; reavaliar quando houver cache TS relevante |
| Migrations DB | **Atlas** (`ariga.io/atlas`) | declarativo, suporta Postgres bem, melhor que goose/migrate puros |
| ORM/SQL Go | **sqlc** | type-safe sem reflection, gera código a partir de SQL puro |
| ORM TS | **Prisma** (mantém escolha do legado) | mais maduro do ecossistema Nest |
| Validação Go | `go-playground/validator/v10` + DTOs gerados de OpenAPI | |
| Validação TS | **Zod** + `nestjs-zod` | já usado no `gateway-web`, unifica web ↔ api |
| Codegen contratos | `oapi-codegen` (Go) + `openapi-ts` (TS) + `quicktype` (eventos Avro→TS/Go) | |
| Linting | `golangci-lint` + `eslint` (flat config) + `prettier` + `ruff` (se houver Python no futuro) | |
| Format | `gofumpt` + `prettier` | |
| Pre-commit | `lefthook` | mais leve que husky, multi-linguagem |
| CI | GitHub Actions com matriz por app + filtros `paths-filter` | |

**Comando único de bootstrap (`make bootstrap`):** instala deps yarn + sobe stack Compose (Postgres + Kafka). Migrations, seeds e geração de contratos entram conforme as Fases 1-2 do roadmap.

---

## 8. Por que essa arquitetura escala

| Pressão de escala | Como o desenho responde |
|-------------------|-------------------------|
| **Pico de transações** | Outbox + Kafka particionado por `merchant_id` permite paralelizar consumidores; Postgres absorve writes locais (transação curta), eventos saem assíncronos. |
| **Novo método de pagamento (Pix Automático, Open Finance)** | Novo adapter em `libs/payments/infra/acquirers/`, registrado via DI. Domínio não muda. |
| **Nova regra antifraude sem deploy** | Specification carregada de tabela versionada. ML model evaluator entra como mais uma Specification. |
| **Compliance/PCI** | Tokenização isola escopo. Apenas o vault (ou adquirente) lida com PAN — `apps/api` fica fora do PCI scope. |
| **Multi-tenant (vários merchants)** | `merchant_id` no path/header → row-level security no Postgres + filtro implícito em todos os repositórios via context. |
| **Observabilidade em incidente** | Trace ID propaga do `apps/web` até o adquirente. Outbox ID + event ID dão idempotência rastreável. |
| **Mudança de stack futura** | Domínio puro permite trocar chi por gRPC, Postgres por CockroachDB, Kafka por NATS sem reescrever regras. |

---

## 9. O que NÃO está no MVP

- Vault próprio de tokenização (usar provedor do adquirente)
- ML model serving (fraud usa só regras determinísticas no v1)
- Multi-região / DR ativo-ativo
- Console admin completo (apenas painel de merchant)
- Plugins de pagamento como sandbox isolado
- Saga orchestrator dedicado (Temporal/Cadence) — Outbox+Inbox cobrem o MVP

Esses entram no roadmap pós-MVP em [`roadmap.md`](./roadmap.md).
