# ADR-0001 — DDD estratégico + Hexagonal por bounded context

- **Status:** Accepted
- **Data:** 2026-05-09
- **Deciders:** time fundador
- **Contexto relacionado:** [`../architecture.md`](../architecture.md)

## Contexto

Os três repos legados (`go-gateway`, `anti-fraud`, `gateway-web`) compartilham domínio (pagamentos) mas mantêm modelos divergentes — `Account` no Go não tem `isSuspicious`, `InvoiceStatus` tem 3 valores no Go e 2 no Nest, payloads de Kafka misturam `snake_case` e `camelCase`. O custo de manter consistência manual entre eles inviabiliza qualquer roadmap de novos métodos de pagamento.

Precisamos consolidar no monorepo `iris-gate` uma arquitetura que:
1. permita evoluir cada contexto (pagamentos, fraude, identidade, ledger) com pouco acoplamento;
2. mantenha o domínio independente de framework (chi/Nest/Next podem mudar);
3. seja auditável por compliance (PCI-DSS) — clareza de onde dado sensível trafega;
4. escale tanto o time (vários owners) quanto o tráfego (sharding por merchant).

## Decisão

Adotar **DDD estratégico** para identificar bounded contexts (Identity, Billing, Payments, Fraud, Ledger, Notifications) e **arquitetura hexagonal/ports-and-adapters** dentro de cada `libs/<context>/`, com layout fixo:

```
libs/<context>/
├── domain/        entidades, value objects, eventos, interfaces de repos, erros
├── application/   use-cases, ports inbound/outbound, policies
└── infra/         adapters: persistence (sqlc/Prisma), http, kafka, acquirers, observability
```

Regra de dependência: `infra → application → domain`. `domain/` não importa nada de fora do próprio pacote.

Este layout já está scaffoldado em `libs/identity/` e `libs/billing/` — confirmado pela exploração.

## Alternativas consideradas

### A. Layered tradicional (handler → service → repo) — abandonada
É o que o `go-gateway` legado usa. Vantagem: simples. Desvantagem: domínio importa `time`, `uuid`, `sync.RWMutex`; lógica de negócio espalha entre service e domain; troca de framework exige reescrita.

### B. Microserviços puros desde o dia 1 — adiada para pós-MVP
Cada bounded context vira app separado. Custo operacional (deploy, tracing distribuído, schemas) alto demais para 1-2 devs. Decisão: começar com 3 apps (`web`, `api-gateway`, `anti-fraud`) onde `api-gateway` agrupa Identity+Billing+Payments+Ledger+Notifications, mas os contextos já vivem em `libs/` separadas — extrair em serviço próprio é mecânico depois.

### C. Modular monolith sem hexagonal — abandonada
Pular as camadas e usar só pacotes Go bem nomeados. Vantagem: menos código. Desvantagem: PCI-DSS exige rastreabilidade clara de onde dado sensível entra/sai; sem ports explícitas, auditoria fica difícil.

## Consequências

### Positivas
- Domínio testável sem subir DB nem broker.
- Trocar adquirente = novo arquivo em `libs/payments/infra/acquirers/`, zero mudança em `domain/`.
- Extração futura de contexto para serviço próprio é mecânica (já está isolado).
- Onboarding mais rápido: novo dev lê o `domain/` para entender as regras antes do `infra/`.

### Negativas
- Mais arquivos / mais imports comparado ao layered.
- Curva de aprendizado para devs sem experiência em DDD.
- Risco de over-engineering em contextos triviais (mitigação: `notifications/` pode ter `application/` mínimo).

### Mitigações
- Gerador de scaffold em `tools/scripts/new-context.sh` para evitar boilerplate manual.
- Code review obrigatório: PR que importa `pgx` ou `chi` em `domain/` é bloqueado.
- ADR-0002 (a escrever) define exatamente o que pode importar onde.
