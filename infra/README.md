# infra

Local development stack. Today the only thing in this folder is Docker Compose under `infra/docker`. There is no cloud definition yet.

```text
infra/
├── docker/
│   ├── docker-compose.yml    the stack
│   ├── Makefile              stack targets (the root Makefile delegates here)
│   ├── .env.example          ports and credentials, copied to .env by `make up`
│   ├── postgres/initdb/      SQL run once when the Postgres volume is created
│   └── kafka/                topics.txt (topic list) and create-topics.sh (run by kafka-init)
├── pki/                      phase 14: local dev CA and mTLS certificates (generated files are gitignored)
└── terraform/                phase 21: AWS deployment
```

Each service that needs files of its own gets a folder under `docker/` named after the service (phase 14 adds `docker/openbao/`). Infrastructure files go here; developer scripts go to `tools/`.

From the repo root, `make up` copies `infra/docker/.env.example` to `.env` when that file is missing, then runs `docker compose up -d --wait`.

```bash
make up
make ps
```

`apps/api` and `apps/web` stay on the host. The anti-fraud process is a service in this Compose file.

> [!IMPORTANT]
> Defaults are for localhost: Postgres user, password, and database are `iris`. Kafka is PLAINTEXT with no auth. Do not publish these ports.

## Services

| Container         | Image                                       | Host port | Role                                                                                        |
| ----------------- | ------------------------------------------- | --------- | ------------------------------------------------------------------------------------------- |
| `iris-postgres`   | `postgres:18.3-alpine`                      | `5432`    | Database `iris` for the API. Init SQL enables `pgcrypto`, `uuid-ossp`, and `citext`         |
| `iris-kafka`      | `apache/kafka:latest`                       | `9092`    | Single-node KRaft broker. No ZooKeeper. Auto-create is off. Log retention is 24 h or 512 MB |
| `iris-kafka-init` | `apache/kafka:latest`                       | none      | One-shot. Runs `kafka/create-topics.sh` over `kafka/topics.txt`, then exits                 |
| `iris-karapace`   | `ghcr.io/aiven-open/karapace:6.1.4`         | `8081`    | Schema registry, Confluent-compatible API, compatibility `BACKWARD`                         |
| `iris-anti-fraud` | built from `apps/anti-fraud/Dockerfile.dev` | `3001`    | Nest worker. Source and Prisma files are mounted for reload                                 |

Logs use the json-file driver, 10 MB × 3 files.

Kafka has two listeners. Inside the Compose network clients use `kafka:9092`. On the host, including `apps/api`, they use `localhost:9092`, which is the `PLAINTEXT_HOST` listener. A host client pointed at `kafka:9092` will not resolve that name.

`kafka-init` creates the topics listed in [`docker/kafka/topics.txt`](docker/kafka/topics.txt) (3 partitions, except the DLQ which has 1). Replication factor is 1. `--if-not-exists` makes a second `make up` safe.

```text
payments.payment_intent.created.v1
payments.payment_intent.authorized.v1
payments.payment_intent.captured.v1
fraud.assessment.requested.v1
fraud.assessment.completed.v1
fraud.assessment.failed.dlq
```

A new event contract needs a line in `docker/kafka/topics.txt`. The broker will not invent the topic.

Karapace health is `GET http://localhost:8081/_health`. Schemas are stored in the Kafka topic `_schemas`. Nothing in the apps registers a schema with it yet.

## Databases

`POSTGRES_DB` is `iris`. `apps/api` runs its own SQL migrations against that database on startup.

The anti-fraud service expects a second database, `antifraud` (`ANTI_FRAUD_DB`), on the same Postgres. Compose does not create it and does not run Prisma migrations. After the first `make up`:

```bash
make psql
```

Then, inside `psql`:

```sql
CREATE DATABASE antifraud OWNER iris;
```

Apply the Prisma migration from the repo root with `make -C apps/anti-fraud prisma-deploy`. The worker's `DATABASE_URL` inside Compose already points at `postgres:5432/antifraud`.

## Environment

`infra/docker/.env.example` is copied to `.env` by `make up`. Compose substitutes these names. Unset values fall back to the defaults in `docker-compose.yml`.

| Variable            | Default                   | Effect                                                                |
| ------------------- | ------------------------- | --------------------------------------------------------------------- |
| `POSTGRES_USER`     | `iris`                    | role and `pg_isready` user                                            |
| `POSTGRES_PASSWORD` | `iris`                    | role password                                                         |
| `POSTGRES_DB`       | `iris`                    | database created by the image                                         |
| `POSTGRES_PORT`     | `5432`                    | host port                                                             |
| `KAFKA_PORT`        | `9092`                    | host port and the advertised host listener                            |
| `KAFKA_CLUSTER_ID`  | `iris-local-cluster-0001` | KRaft cluster id. Changing it on an existing volume breaks the broker |
| `KARAPACE_PORT`     | `8081`                    | host port                                                             |
| `ANTI_FRAUD_PORT`   | `3001`                    | host port                                                             |
| `ANTI_FRAUD_DB`     | `antifraud`               | database name in the worker URL                                       |

The anti-fraud service sets `KAFKA_BROKERS=kafka:9092`. Its consumer group is `KAFKA_CONSUMER_GROUP`, which Compose leaves unset, so the code default `iris-anti-fraud` applies. Details are in `apps/anti-fraud/README.md`.

## Commands

From the repo root. The same targets exist in `infra/docker/Makefile` if you are already in that directory.

| Target              | Effect                                                        |
| ------------------- | ------------------------------------------------------------- |
| `make up`           | copy `.env` if needed, start the stack, wait until healthy    |
| `make down`         | stop containers, keep volumes                                 |
| `make restart`      | `down` then `up`                                              |
| `make ps`           | container status                                              |
| `make logs`         | follow the last 100 lines                                     |
| `make psql`         | `psql` in `iris-postgres` as `POSTGRES_USER` on `POSTGRES_DB` |
| `make kafka-topics` | list topics                                                   |
| `make nuke`         | ask for confirmation, then `docker compose down -v`           |

Inside `infra/docker`, `make pull` refreshes images, `make pg-dump` prints the `iris` schema, `make kafka-shell` opens a shell in the broker, and `make kafka-create-topic NAME=... [PARTITIONS=1] [RF=1]` adds a topic by hand.

`make nuke` deletes Postgres and Kafka data. The next `make up` starts from empty volumes, and the `antifraud` database has to be created again.
