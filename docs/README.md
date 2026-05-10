# IrisGate — Documentação de Arquitetura

> Gateway de pagamentos API-first. Roteamento dinâmico, observabilidade ponta-a-ponta, motor de risco plugável.
> Stack: **Next.js (web)** · **Go (api-gateway)** · **NestJS (anti-fraud)** · **Kafka (eventos)** · **PostgreSQL (estado)**.

## Índice

| #   | Documento                              | Conteúdo                                                                                                      |
| --- | -------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| 1   | [`legacy-audit.md`](./legacy-audit.md) | Auditoria dos repos legados (`go-gateway`, `anti-fraud`, `gateway-web`). O que reaproveitar, o que descartar. |
| 2   | [`architecture.md`](./architecture.md) | Arquitetura alvo: bounded contexts, camadas DDD/hexagonal, fluxo de eventos, contratos.                       |
| 3   | [`security.md`](./security.md)         | Baseline de segurança para pagamentos (PCI-DSS, tokenização PAN, idempotência, segredos, mTLS).               |
| 4   | [`roadmap.md`](./roadmap.md)           | Roadmap em fases — do esqueleto atual ao MVP produtivo.                                                       |
| 5   | [`adr/`](./adr/)                       | Architecture Decision Records. Decisões irreversíveis ficam aqui.                                             |

## TL;DR — crítica em 5 linhas

1. **`go-gateway`** tem layered arch razoável e Kafka funcionando, mas **trata PAN em plaintext, API key sem hash, zero idempotência, fraude mockada com `random.Float64()`, e Kafka+DB não atômico** (publica fora de transação).
2. **`anti-fraud`** tem o melhor ativo de todos: o **Specification Pattern** para regras é mantenedor. Mas é Nest "vanilla" sem DDD, sem DLQ, sem auth entre serviços, e o consumer não é idempotente (redelivery duplica decisão).
3. **`gateway-web`** tem boa base shadcn + RHF/Zod, mas **duplica tipos do backend manualmente**, login guarda `apiKey` em cookie sem `httpOnly`, e features (PDF, detalhe da invoice) ficaram pela metade.
4. **`iris-gate` (atual)** é só esqueleto: tem `libs/{identity,billing,shared}` com layout hexagonal pronto, mas **zero toolchain** (sem `package.json`, `go.work`, `pnpm-workspace.yaml`, `turbo.json`, `Makefile`) e só `apps/api-gateway` — faltam `apps/web` e `apps/anti-fraud`.
5. **Caminho proposto**: bounded contexts explícitos (Identity, Billing, Payments, Fraud, Ledger, Notifications), Outbox+Inbox para atomicidade DB↔Kafka, tokenização de PAN antes de qualquer persistência, Argon2id em API keys, OpenTelemetry desde o dia zero, contratos versionados em `libs/contracts/` (OpenAPI + AsyncAPI) gerando tipos para Go, TS e Nest.

Detalhes em cada doc.
