# ADR-0002 — Outbox/Inbox para atomicidade DB ↔ Kafka

- **Status:** Accepted
- **Data:** 2026-05-09
- **Deciders:** time fundador
- **Contexto relacionado:** [`../architecture.md`](../architecture.md), [`../legacy-audit.md`](../legacy-audit.md)

## Contexto

A auditoria do `go-gateway` legado encontrou um caso clássico de **dual-write**: `InvoiceService.Create()` faz `repo.Save(invoice)` e depois `kafkaProducer.Publish(...)` fora de transação. Cenários de falha:

| Falha | Resultado |
|-------|-----------|
| Save OK, Publish falha | Invoice no DB sem evento — anti-fraud nunca avalia, status fica eterno em `pending` |
| Save falha, Publish foi feito antes | Anti-fraud avalia uma invoice que não existe |
| Crash entre Save e Publish | Mesmo cenário 1 |

A auditoria do `anti-fraud` legado encontrou o problema simétrico: o consumer **não é idempotente** — Kafka redelivery (acontece em rebalance, em retry de commit) reprocessa a mensagem e cria `FraudHistory` duplicado.

Para um gateway de pagamentos, esses cenários são inaceitáveis.

## Decisão

Adotar **Transactional Outbox** para todo write que precisa publicar evento, e **Idempotent Inbox** para todo consumer.

### Outbox (lado producer)

Tabela por contexto:
```sql
CREATE TABLE outbox_events (
  id              UUID PRIMARY KEY,
  aggregate_type  TEXT NOT NULL,           -- 'PaymentIntent'
  aggregate_id    UUID NOT NULL,
  event_type      TEXT NOT NULL,           -- 'payments.intent.created.v1'
  payload         JSONB NOT NULL,
  headers         JSONB NOT NULL,          -- traceparent, idempotency-key, ...
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at    TIMESTAMPTZ,
  attempts        INT NOT NULL DEFAULT 0,
  last_error      TEXT
);
CREATE INDEX outbox_unpublished ON outbox_events (created_at) WHERE published_at IS NULL;
```

Use-case escreve `aggregate` + `outbox_events` na **mesma transação**. Worker dedicado (`libs/platform/outbox/relay`) faz polling (`200ms`) ou usa `LISTEN/NOTIFY`, publica em Kafka, marca como `published_at`. Em falha, incrementa `attempts` e tenta de novo com backoff. Após N tentativas, alerta.

### Inbox (lado consumer)

Tabela compartilhada por consumer group:
```sql
CREATE TABLE inbox_messages (
  message_id      TEXT NOT NULL,           -- event_id do header
  consumer        TEXT NOT NULL,           -- 'fraud-assessment-consumer'
  processed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (message_id, consumer)
);
```

Cada mensagem é envolvida em transação:
```
BEGIN
  INSERT INTO inbox_messages (message_id, consumer)
    VALUES ($1, $2)
    ON CONFLICT DO NOTHING
    RETURNING 1;
  -- se RETURNING vazio: já foi processado, COMMIT e descarta
  -- senão: aplica side-effect (escreve fraud_history, publica resultado via outbox)
COMMIT
```

DLQ separado (`<topic>.dlq`) para mensagens que falham N vezes.

## Alternativas consideradas

### A. Saga orchestrator (Temporal/Cadence)
Resolveria atomicidade e mais. Custo operacional (mais um serviço com state) e curva de aprendizado altos para o MVP. **Decisão:** adiar para pós-MVP quando workflows multi-step ficarem comuns. Outbox+Inbox cobrem o caso de 1 write + 1 evento.

### B. Kafka Transactions (exactly-once semantics nativo)
Kafka tem suporte a transações cross-topic, mas integração com transação Postgres exige 2-phase commit (XA), que tem fama de instável. Outbox é mais simples e funciona com qualquer broker.

### C. Change Data Capture (Debezium)
Lê o WAL do Postgres e publica eventos automaticamente. Atrativo, mas: (1) acopla schema do DB ao schema do evento; (2) operacionalmente pesado (Kafka Connect, snapshots iniciais); (3) eventos refletem estrutura tabular, não a linguagem ubíqua do domínio.

### D. Event Store dedicado (EventStoreDB, Marten)
Source-of-truth = stream de eventos. Atraente para Ledger, exagero para o resto. **Decisão:** considerar para Ledger no pós-MVP; usar Postgres + Outbox em todo o resto.

## Consequências

### Positivas
- Atomicidade real: ou ambos (state + evento) acontecem, ou nenhum.
- Recuperação de falha trivial: relay worker reinicia e drena o backlog.
- Idempotência garantida: redelivery é seguro.
- Funciona com qualquer broker (Kafka, Redpanda, NATS, RabbitMQ).

### Negativas
- Latência adicional: relay polling adiciona até `200ms` de delay médio. Mitigável com `LISTEN/NOTIFY`.
- Mais uma tabela por contexto + um worker por app.
- Necessário monitorar lag do outbox (alerta se `count(*) where published_at IS NULL > limiar`).

### Métricas a expor
- `outbox_unpublished_count` (gauge)
- `outbox_publish_latency_ms` (histogram)
- `inbox_dedupe_hits_total` (counter, ajuda a entender se redelivery está alto)
- `dlq_messages_total` (counter por tópico)
