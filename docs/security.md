# Baseline de Segurança — IrisGate

> Pagamentos não perdoam falha de segurança. Este documento define o **mínimo não-negociável** para o MVP. Qualquer PR que viole um item aqui é bloqueado em code review.

---

## 1. Princípios

1. **Defense in depth** — nenhuma camada confia em outra cegamente.
2. **Least privilege** — todo serviço, role, secret, conexão DB tem apenas o escopo estritamente necessário.
3. **Fail closed** — em dúvida sobre uma transação, decisão padrão é `REVIEW` ou `REJECT`, nunca `APPROVE`.
4. **Auditável por design** — toda decisão sensível (autorização, mudança de chave, alteração de regra de fraude) gera evento em log append-only com `actor`, `timestamp`, `trace_id`.
5. **Secrets nunca em código** — nenhum `.env` versionado com valores reais; use Vault/SOPS/Doppler.

---

## 2. Dados de cartão (PAN, CVV, expiração)

### Regra de ouro
**`apps/api-gateway` jamais recebe, processa ou loga PAN.**

### Implementação
- Frontend (`apps/web`) chama **direto** o endpoint de tokenização do adquirente (via JS SDK do provedor) e recebe um `card_token` opaco.
- O backend recebe somente `card_token`, `last_four`, `brand`, `exp_month`, `exp_year`.
- CVV **nunca persistido** em nenhuma camada. Trafega só na call síncrona ao adquirente, e mesmo isso é evitado quando o token já carrega autorização pré-validada.
- Logs estruturados têm allowlist de campos. PAN, CVV, `card_token` (sim, mesmo o token) **não vão para log**.

### Validação automatizada
- Lint customizado em CI: regex `\b\d{13,19}\b` em logs/responses → falha o build.
- Teste de contrato: nenhuma rota OpenAPI pode ter `pan`, `cardNumber`, `cvv` no schema de request/response.

### Escopo PCI-DSS
Mantendo PAN fora do `api-gateway`, o escopo PCI fica restrito ao **vault do adquirente**. Isso reduz o esforço de compliance de meses para semanas.

---

## 3. Credenciais de merchants (API keys)

### Geração
- 32 bytes de entropia via `crypto/rand` (Go) → encoded em `base64url`.
- Prefixo identificável: `irk_live_...` ou `irk_test_...` (igual Stripe). Permite revogação rápida ao vazar em log público (busca por prefixo).

### Persistência
- **Hash com Argon2id** (params: `m=64MB, t=3, p=2`). Nunca em plaintext.
- Schema:
  ```sql
  CREATE TABLE api_keys (
    id            UUID PRIMARY KEY,
    merchant_id   UUID NOT NULL REFERENCES merchants(id),
    prefix        TEXT NOT NULL,           -- 'irk_live_aBc12'  (12 chars, indexável)
    hash          TEXT NOT NULL,           -- argon2id full string
    last_used_at  TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ,
    expires_at    TIMESTAMPTZ,             -- rotação obrigatória 90 dias
    scopes        TEXT[] NOT NULL,         -- ['payments:write','payments:read']
    created_at    TIMESTAMPTZ NOT NULL
  );
  CREATE INDEX api_keys_prefix ON api_keys (prefix) WHERE revoked_at IS NULL;
  ```
- Lookup: parse do header → extrai prefix → fetch row pelo prefix → `argon2id.Verify(hash, key)`. Pula full-table scan.

### Rotação
- Endpoint `POST /api-keys/rotate` cria nova chave, marca antiga com `expires_at = now() + 7d` (grace period).
- Painel mostra alerta para chaves > 60 dias.

### Painel admin
- Auth do **painel** é separada da API (OIDC com provider externo: Clerk, Auth0, ou Keycloak self-hosted). API keys são para uso máquina-a-máquina apenas.

---

## 4. Autenticação entre serviços (api-gateway ↔ anti-fraud)

Problema do legado: qualquer serviço com acesso ao broker conseguia publicar `pending_transactions` ou consumir `transaction_results`.

### Solução
- **Kafka SASL/SCRAM-SHA-512** + **TLS** entre serviços e broker. Cada serviço tem usuário próprio com ACL por tópico:
  ```
  api-gateway:  WRITE payments.*, fraud.assessment.requested.*
                READ  fraud.assessment.completed.*
  anti-fraud:   READ  fraud.assessment.requested.*
                WRITE fraud.assessment.completed.*
  ```
- **Eventos assinados**: header `signature` = `HMAC-SHA256(payload, shared_secret_per_topic)`. Consumer valida antes de processar. Secret rotaciona via Vault sem downtime (dual-secret window).
- Para gRPC interno (caminho síncrono raro): **mTLS** com cert-manager + CA interna.

---

## 5. Autenticação do painel web (apps/web)

### Decisão
**API key do merchant nunca toca o navegador.** O painel autentica usuários (humanos) com OIDC; cada usuário pertence a 1+ merchants com role.

### Fluxo
```
1. user → /auth/login                  redireciona pro IdP (OIDC)
2. IdP → callback                       cookie de sessão Iron-Session (httpOnly, Secure, SameSite=Strict)
3. user → server action (Next)          server lê sessão, busca API key do merchant via Vault
4. server action → api-gateway          envia X-API-Key (server-side only)
5. resposta → cliente                   cliente nunca vê a API key
```

### Cookies
- `httpOnly: true`
- `secure: true` (sempre, em dev usa `mkcert` para HTTPS local)
- `sameSite: 'strict'`
- `maxAge: 1h` com refresh silencioso
- Path `/`, sem subdomínio amplo

### CSP
Header `Content-Security-Policy` restritivo: `default-src 'self'; script-src 'self' <hash>; ...`. Sem `unsafe-inline`.

---

## 6. Idempotência

| Camada | Mecanismo |
|--------|-----------|
| **HTTP write** | Header obrigatório `Idempotency-Key` em todo `POST/PUT/DELETE` mutativo. TTL 24h. Hash do request body verificado em replay. |
| **Kafka consumer** | Tabela `inbox(message_id, consumer)` com PK composta. INSERT-ON-CONFLICT-DO-NOTHING antes de side-effect. |
| **Webhook outbound** | Cada delivery tem ID único. Merchant deve tratar como idempotente. Documentado no portal. |

---

## 7. Validação de entrada

### REST inbound
- DTOs gerados de OpenAPI 3.1 (oapi-codegen no Go, nestjs-zod no Nest).
- Validação na borda — se inválido, 422 antes de tocar use-case.
- Sem `interface{}` / `any` nos handlers.

### Kafka inbound
- Schema validation contra Avro/JSON Schema do registry. Mensagem inválida → DLQ com motivo.

### Sanitização de query
- Apenas `sqlc` (parameterized queries) — zero string concat para SQL.
- ORM Prisma já é parameterized.

---

## 8. Rate limiting + abuse

- **API pública (api-gateway)**: rate limit por API key, sliding window via Redis. Default `100 req/s` por key. Override por merchant em tabela `rate_limit_overrides`.
- **Painel (apps/web)**: rate limit por sessão + por IP no middleware Next.
- **Login OIDC**: delegado ao IdP (eles já fazem brute-force protection).
- **Headers de resposta**: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`.

---

## 9. Logs e PII

### Allowlist de campos para log estruturado
```
SAFE:    trace_id, span_id, merchant_id, intent_id, status, latency_ms, error_code, http_status
NEVER:   pan, cvv, card_token, api_key, password, customer_email (sem hash), customer_doc, ip (sem hash)
HASH:    customer_email → SHA-256 truncated, ip → SHA-256(ip + daily_salt)
```

### Logger
- Go: `slog` com handler customizado que dropa keys da denylist.
- Nest: `pino` com `redact` configurado.
- CI: regex check em snapshots de log de teste.

### Retenção
- Logs de aplicação: 30 dias hot, 1 ano cold.
- Logs de auditoria (decisões de fraude, mudanças de chave): **7 anos**, append-only, em bucket WORM.

---

## 10. Secrets management

| Tipo | Onde |
|------|------|
| Dev local | `.env.local` ignorado pelo git, valores fake. `.env.example` versionado. |
| CI | GitHub Actions secrets, escopo por environment. |
| Staging/Prod | Vault (HashiCorp) ou Doppler. Serviços leem via sidecar/agent, nunca via env file no container. |
| Rotação | Chaves de DB, Kafka, signing keys → 90 dias. Process documentado em runbook. |

### Detecção de leak
- `gitleaks` no pre-commit (lefthook).
- `gitleaks` em CI obrigatório.
- Alerta no Sentry/Datadog quando log contém pattern de secret.

---

## 11. HTTP hardening (apps/api-gateway e apps/web)

| Header | Valor |
|--------|-------|
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains; preload` |
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `geolocation=(), microphone=(), camera=()` |
| `Content-Security-Policy` | restritivo (ver §5) |

CORS: allowlist explícita por origem. Wildcard `*` proibido.

---

## 12. Banco de dados

### Conexões
- TLS obrigatório (`sslmode=require` no Postgres).
- Usuário por serviço, com permissões mínimas (`api-gateway` não tem `DROP`, `anti-fraud` não acessa `payments.*`).
- Connection pool com timeout agressivo (5s connect, 30s query).

### Row-level security
- `merchants` table com RLS habilitado. Policy: `current_setting('app.merchant_id') = merchant_id`.
- `SET LOCAL app.merchant_id = $1` na borda HTTP, antes de qualquer query.

### Backups
- Snapshot diário + WAL contínuo.
- Restore drill mensal documentado.
- Encriptação at-rest (KMS gerenciado).

---

## 13. Dependências

- `pnpm audit` + `govulncheck` em CI.
- `dependabot` para PRs de update.
- Lockfile commitado e checado em CI.
- Imagem base Docker: `distroless` ou `scratch` quando possível. Sem shell em prod.

---

## 14. Threat model resumido (STRIDE)

| Ameaça | Vetor | Mitigação |
|--------|-------|-----------|
| **S**poofing de merchant | API key roubada | Argon2id hash, scopes, prefix p/ revogação rápida, alerta de uso anômalo |
| **T**ampering de evento Kafka | Acesso ao broker | TLS + SASL + HMAC por evento + ACL por tópico |
| **R**epudiation | Merchant nega ter autorizado refund | Audit log append-only com trace_id + actor + IP hash |
| **I**nformation disclosure | PAN em log | Tokenização end-to-end, logger com denylist, lint regex em CI |
| **D**enial of Service | Burst de requisições | Rate limit por key, circuit breaker em adquirentes, autoscale do consumer |
| **E**levation of privilege | Bug em RBAC | Testes de policy obrigatórios, RLS no DB como segunda camada |

---

## 15. Checklist por PR

- [ ] Nenhum log novo expõe campo da denylist (§9)
- [ ] Toda rota mutativa aceita `Idempotency-Key`
- [ ] DTO da rota está em `libs/contracts/http/` e validado
- [ ] Migration nova respeita least-privilege (sem `GRANT ALL`)
- [ ] Se o PR introduz consumer Kafka, tem inbox + DLQ
- [ ] Se o PR introduz produtor Kafka, usa Outbox (não publica fora de transação)
- [ ] Se o PR adiciona dependência, `govulncheck`/`pnpm audit` passou
- [ ] Se o PR toca `payments/`, alguém com `code-owner: payments` revisou
