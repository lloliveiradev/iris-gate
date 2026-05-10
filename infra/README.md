# infra/

Infrastructure-as-code and local dev stacks for iris-gate.

## Layout

```
infra/
  docker/        # local dev compose (postgres, kafka, ...)
```

## Local stack

Postgres 18.3 + Apache Kafka (KRaft, single node — sem Zookeeper) + Karapace (schema registry, compatível com Confluent Schema Registry API).

```bash
cd infra/docker
cp .env.example .env
docker compose up -d
docker compose ps
```

Endpoints:
- Postgres → `localhost:5432` (user/db: `iris`)
- Kafka    → `localhost:9092` (PLAINTEXT, auto-create topics disabled)
- Karapace → `localhost:8081` (schema registry, compatibility `BACKWARD`)

Tear down (keeps volumes):
```bash
docker compose down
```

Wipe everything (drops data):
```bash
docker compose down -v
```
