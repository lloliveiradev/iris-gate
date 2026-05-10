# Auditoria dos Repositórios Legados

Auditoria conduzida em 2026-05-09 sobre os três repos que precedem o monorepo `iris-gate`. Cada seção lista o que **reaproveitar conceitualmente** (não copiar literalmente) e o que **descartar/reescrever**.

---

## 1. `go-gateway` (Go + chi + Postgres + Kafka)

### Estrutura observada

```
cmd/app/main.go              entry-point
internal/domain/             Account, Invoice, CreditCard, events/
internal/service/            AccountService, InvoiceService, KafkaProducer/Consumer
internal/repository/         AccountRepository, InvoiceRepository (SQL raw)
internal/web/handlers/       AccountHandler, InvoiceHandler
internal/web/middleware/     AuthMiddleware (X-API-Key)
internal/web/server/         chi router, CORS
internal/dto/                request/response shapes
migrations/                  *.up.sql (sem runner)
```

### Modelo de domínio
- **Account** — `id, name, email (unique), api_key (unique, plaintext), balance (decimal), timestamps`. Geração de chave via `crypto/rand` (OK), persistência em texto puro (não OK).
- **Invoice** — `id, account_id, amount, status {pending|approved|rejected}, payment_type, card_last_digits, timestamps`. Método `Process()` decide aprovação **com `rand.Float64()`** se `amount < 10k`; acima disso, fica pending para o Kafka decidir.
- **CreditCard** — recebido no request, **nunca persistido inteiro** (só os últimos 4 dígitos). Mas trafega por handler→service→domínio em plaintext.

### Comunicação Kafka
- Producer publica `PendingTransaction` no tópico `pending_transactions` quando `amount >= 10k`.
- Consumer lê `transaction_results`, atualiza status da invoice via `InvoiceService.ProcessTransactionResult`.
- Loop infinito sem `context.Cancel`, sem signal handling, sem graceful shutdown.

### Endpoints HTTP
| Método | Rota | Auth |
|--------|------|------|
| POST | `/accounts` | público |
| GET | `/accounts` | X-API-Key |
| POST | `/invoices` | X-API-Key |
| GET | `/invoices/{id}` | X-API-Key |
| GET | `/invoices` | X-API-Key |
| GET | `/invoices/filter?status&start_date&end_date` | X-API-Key |

### Pontos críticos

| Severidade | Problema | Onde |
|------------|----------|------|
| **CRÍTICO** | PAN em memória plaintext (request → domínio → service) | `internal/domain/invoice.go` |
| **CRÍTICO** | API key persistida em plaintext | `internal/domain/account.go` |
| **CRÍTICO** | Sem idempotência em `POST /invoices` | `internal/web/handlers/invoice_handler.go` |
| **CRÍTICO** | `Save → Kafka.Publish` não-atômico (DB ok, Kafka falha = inconsistência permanente) | `internal/service/invoice_service.go` |
| Alto | Decisão de aprovação por `random.Float64()` | `internal/domain/invoice.go:Process` |
| Alto | Zero testes (`*_test.go` ausente em todo repo) | — |
| Alto | Sem migrations runner (golang-migrate, atlas, goose) | `migrations/` |
| Médio | Domínio importa `sync.RWMutex`, `time.Time`, `uuid` — vazamento de infra | `internal/domain/account.go` |
| Médio | Handler retorna `http.Error(500)` para qualquer erro de service | handlers/* |
| Médio | Sem observabilidade (sem request id, sem trace, sem métricas) | `cmd/app/main.go` |
| Médio | Sem graceful shutdown do consumer Kafka | `cmd/app/main.go` |

### Veredito
**Reaproveitar conceitualmente:** divisão domain/service/repository/handler, padrão API key + middleware, esqueleto Kafka producer/consumer.
**Reescrever do zero:** persistência de PAN (→ tokenização), credenciais (→ Argon2id), atomicidade (→ Outbox), motor de aprovação (→ delegar 100% ao serviço de fraude), todo pacote `domain/` desacoplado de `time`/`uuid`/`sync`.

---

## 2. `anti-fraud` (NestJS + Prisma + KafkaJS)

### Estrutura observada

```
src/invoices/                controller, service, consumer
src/invoices/fraud/          fraud.service, specifications/*
src/kafka/                   KafkaServer (Nest server custom), KafkaService
src/events/                  EventEmitter2 publisher
src/prisma/                  PrismaService wrapper
prisma/schema.prisma         Account, Invoice, FraudHistory
```

### Schema Prisma
```
Account      { id, isSuspicious: Boolean, timestamps, invoices[] }
Invoice      { id, accountId, amount: Float, status: enum, fraudHistory? }
FraudHistory { id, invoiceId @unique, reason: enum, description, timestamps }

enum InvoiceStatus { APPROVED | REJECTED }
enum FraudReason   { SUSPICIOUS_ACCOUNT | UNUSUAL_PATTERN | FREQUENT_HIGH_VALUE }
```
**O que falta:** score numérico, weights por regra, snapshot estruturado das features avaliadas, timestamp da última avaliação, blocklist persistente.

### Motor de fraude — **o melhor ativo do legado**
Specification Pattern com agregador:

```
FraudAggregateSpecification
 ├─ SuspiciousAccountSpecification     (account.isSuspicious)
 ├─ UnusualAmountSpecification         (amount > avg * (1+threshold))
 └─ FrequentHighValueSpecification     (count em janela H > limiar)
```
Primeira regra ativada vence (sem composição de score). Limiares vêm de `ConfigService`. Adicionar nova regra = nova classe + registrar no `FRAUD_SPECIFICATIONS` token. **Manter esse padrão.**

### Comunicação Kafka
- `KafkaServer` custom estende `Server` do Nest e usa KafkaJS direto (não `@nestjs/microservices` transport).
- Consome `pending_transactions`, produz em `transaction_results` via `KafkaService`.
- Sem DLQ. Sem retry com backoff. Sem idempotência (consumer não checa se `invoice_id` já tem `FraudHistory`).

### Pontos críticos

| Severidade | Problema | Onde |
|------------|----------|------|
| **CRÍTICO** | Consumer não-idempotente — redelivery cria histórico duplicado | `src/invoices/invoices.consumer.ts` |
| **CRÍTICO** | Sem auth entre serviços (qualquer um que alcance o broker publica `pending_transactions`) | `src/kafka/*` |
| Alto | Sem DLQ — falha no consumer perde mensagem | `src/kafka/kafka-server.ts` |
| Alto | Sem circuit breaker — falha do Kafka derruba app no `onModuleInit` | `src/kafka/kafka.service.ts` |
| Alto | Regras hardcoded como código (sem hot-reload, sem A/B, sem versionamento) | `src/invoices/fraud/specifications/` |
| Alto | Sem `ValidationPipe` global — DTOs com `class-validator` não são validados | `src/main.ts` |
| Médio | Controller chama Prisma direto (sem repository port) | `src/invoices/invoices.service.ts` |
| Médio | Score binário (0/1 por regra), sem composição ponderada | `src/invoices/fraud/fraud-aggregate.specification.ts` |
| Médio | Testes apenas `toBeDefined()` | `src/**/*.spec.ts` |

### Veredito
**Reaproveitar conceitualmente:** Specification Pattern, schema Prisma como ponto de partida (precisa enriquecer), `KafkaServer` custom é elegante.
**Reescrever:** consumer → idempotente via tabela `processed_messages`, score → composto e versionado, DTOs → validados globalmente, regras → carregadas de DB ou config dinâmico.

---

## 3. `gateway-web` (Next 15 App Router + shadcn + RHF/Zod)

### Estrutura observada

```
app/auth/                     login (cookie apiKey)
app/dashboard/                listagem invoices
app/dashboard/new-invoice/    formulário cartão
app/dashboard/invoice/[id]/   detalhe (vazio)
app/api/invoice/[id]/pdf/     rota PDF (vazia)
components/                   ~50 shadcn + custom (status-badge, header)
hooks/                        use-mobile, use-toast, usePagination
interfaces/Invoice.ts         DTOs duplicados manualmente
middleware.ts                 protege /dashboard/*
```

### Stack
Next 15.2.4 · React 19 · TS 5 · Tailwind 3.4 · shadcn/ui (Radix) · RHF 7.55 · Zod 3.24 · Sonner · next-themes · Lucide.

### Comunicação backend
- Fetch nativo direto (sem wrapper, sem axios, sem react-query/swr).
- `NEXT_PUBLIC_GATEWAY_API_URL` + cookie `apiKey` + header `X-API-KEY`.
- Server actions em `app/dashboard/actions.ts` e `app/dashboard/new-invoice/actions.ts`.

### Pontos críticos

| Severidade | Problema | Onde |
|------------|----------|------|
| **CRÍTICO** | `NEXT_PUBLIC_GATEWAY_API_URL` expõe URL e o fluxo de login envia API key direto do client → vaza pelos devtools | `app/auth/page.tsx` |
| **CRÍTICO** | Cookie `apiKey` sem `httpOnly` declarado, sem `Secure`, sem `SameSite=Strict` (verificar middleware) | `middleware.ts` |
| Alto | Tipos backend duplicados manualmente — drift garantido | `interfaces/Invoice.ts` |
| Alto | Sem `revalidateTag` (comentado no código) — dados nunca caem em cache controlado | `app/dashboard/actions.ts` |
| Médio | Rota `/dashboard/invoice/[id]/page.tsx` vazia | — |
| Médio | Rota `/api/invoice/[id]/pdf` vazia (libs `pdf-lib`, `html2pdf.js` instaladas e não usadas) | — |
| Médio | Erro genérico em fetch (`{ error: "Falha..." }`) sem tipo | `app/dashboard/actions.ts` |
| Médio | Sem Suspense / loading skeletons no dashboard | — |
| Baixo | `formatCurrency` em `lib/utils.ts` com lógica de cursor — frágil para input controlado | `lib/utils.ts` |

### Veredito
**Reaproveitar:** shadcn config (`components.json`), Tailwind config, paleta `next-themes`, esqueleto RHF+Zod, padrão de server actions.
**Reescrever:** cliente HTTP (criar wrapper tipado e isomórfico em `libs/sdk-web`), tipos (gerar de OpenAPI), auth (Auth.js v5 ou custom com session cookie criptografado server-side, **API key nunca toca o client**).

---

## Síntese cross-repo

```
                  ┌──────────────────────────────────────────┐
                  │         INCONSISTÊNCIAS CHAVE            │
                  └──────────────────────────────────────────┘

  go-gateway              anti-fraud              gateway-web
  ──────────              ──────────              ───────────
  Account.isSuspicious    Account.isSuspicious    (ausente)
   ✗ não existe            ✓ existe                 — owners?

  Invoice.status          Invoice.status          InvoiceStatus
   {pending|approved|      {APPROVED|REJECTED}     (string livre)
    rejected}               ✗ sem PENDING           ✗ sem enum tipado

  PendingTransaction      pendingInvoicesMessage  (não conhece)
   {AccountID,             {invoice_id,             — contrato
    InvoiceID, Amount}      account_id, amount}      não compartilhado
   ✗ snake_case mismatch    ✗ snake_case mismatch
```

**Conclusão:** os três repos têm modelos divergentes do mesmo domínio. O monorepo é a oportunidade de **consolidar contratos** (`libs/contracts/`) e **unificar enums** antes de qualquer linha de código novo.
