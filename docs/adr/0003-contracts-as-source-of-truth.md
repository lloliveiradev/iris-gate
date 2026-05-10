# ADR-0003 — Contratos (OpenAPI + AsyncAPI) como Single Source of Truth

- **Status:** Accepted
- **Data:** 2026-05-09
- **Deciders:** time fundador
- **Contexto relacionado:** [`../architecture.md`](../architecture.md), [`../roadmap.md`](../roadmap.md) (Fase 1)

## Contexto

O `legacy-audit.md` documenta um drift crônico entre os 3 repos legados:
- `Account` em Go sem `isSuspicious`, presente no `anti-fraud`.
- `InvoiceStatus` com 3 valores no Go e 2 no Nest.
- Payloads Kafka misturando `snake_case` (Go) e `camelCase` (Nest).
- Web reconstruindo DTOs à mão a cada endpoint novo, sem garantia de match com backend.

Cada repo possui sua própria definição "de cabeça" do mesmo conceito. Não há instância única que sirva de árbitro. Todo bug de integração começa aqui.

## Decisão

Adotar **contratos versionados em arquivos** dentro de `libs/contracts/` como **única fonte da verdade** para tudo que cruza fronteira (HTTP entre web↔api, eventos entre serviços via Kafka):

- **HTTP**: OpenAPI 3.1 (`libs/contracts/http/<contexto>.openapi.yaml`).
- **Eventos**: AsyncAPI 2.6 + JSON Schema 2020-12 (`libs/contracts/events/`).
- **Tipos gerados** — não escritos à mão — em `libs/contracts/generated/{go,ts}/`, comitados no repo.

### Pipeline

```
libs/contracts/http/*.openapi.yaml ──▶ oapi-codegen ──▶ generated/go/<ctx>/types.gen.go
                                  ╰─▶ openapi-typescript ──▶ generated/ts/<ctx>.d.ts
libs/contracts/events/schemas/**/*.json (futuro: codegen events)
```

Comando único: `make gen-contracts` (raiz). CI: `make gen-contracts-check` falha se a árvore gerada divergir do comitado (gate anti-drift).

### Convenções

- **Naming**: `snake_case` em todos os campos JSON. Tradução pra `camelCase` é responsabilidade da camada client/UI, nunca do contrato.
- **Versionamento HTTP**: rota com `/v1/...`. Breaking change exige `/v2/...` e período de dual-serve.
- **Versionamento eventos**: nome do tópico com sufixo `.vN` (ex.: `identity.merchant.created.v1`). Breaking change cria `.vN+1` com janela de dual-publish.
- **Schema registry** (dev): Karapace, compatibilidade `BACKWARD` por padrão, exceção explícita pra eventos `breaking`.
- **Fonte do contrato manda no código**: editar `*.openapi.yaml` → rodar codegen → atualizar handlers/clients. Nunca o inverso.

### Regra de dependência

`apps/* → libs/contracts/generated/* → libs/contracts/{http,events}/* (texto YAML/JSON)`

`libs/<bounded-context>/domain/` **não** importa de `libs/contracts/`. Os tipos gerados são DTOs de delivery; o domínio tem seus próprios tipos. Mapeamento explícito acontece na camada `infra/http` ou `infra/kafka` de cada contexto.

## Consequências

**Positivas**

- Quebra um campo no OpenAPI → quebram os builds Go (apps/api) e TS (apps/web) no mesmo CI run, antes de qualquer merge.
- Onboarding de cliente externo trivial: spec OpenAPI vira portal automático (Redocly/Swagger).
- Versionamento explícito força conversa sobre breaking changes em vez de silenciosamente quebrar consumidores.
- Eventos auditáveis: schema do payload está versionado em arquivo, não disperso em logs.

**Negativas / mitigações**

- Codegen quebra fluxo "edita e roda" — mitigado por `make gen-contracts` rápido (<2s por linguagem) e pre-commit hook.
- Tipos gerados poluem PRs — mitigado por configurar regra de review: arquivos em `generated/` revistos apenas se a fonte (`http/*.yaml`, `events/*.json`) também mudou.
- Curva inicial do AsyncAPI — mitigado começando com 1 evento e expandindo conforme cada Fase precisa.

## Não decidido aqui

- **Adquirentes externos**: contratos deles não vão pra `libs/contracts/`. Adapters em `libs/payments/infra/acquirers/<nome>/` mantêm seus próprios DTOs.
- **gRPC interno**: se vier no futuro, entra como `libs/contracts/grpc/*.proto` mantendo o mesmo princípio.
- **Mock server / contract tests**: deixado pra ADR futura quando o volume de specs justificar.

## Referências

- OpenAPI 3.1 spec: https://spec.openapis.org/oas/v3.1.0
- AsyncAPI 2.6 spec: https://www.asyncapi.com/docs/reference/specification/v2.6.0
- oapi-codegen: https://github.com/oapi-codegen/oapi-codegen
- openapi-typescript: https://github.com/openapi-ts/openapi-typescript
- Karapace: https://github.com/Aiven-Open/karapace
