# apps/api

API Gateway HTTP em Go (chi v5 + `log/slog`).

## Layout

```
apps/api/
  cmd/api/             # entrypoint (main.go)
  internal/
    config/            # carregamento de env
    httpapi/           # router, handlers, middlewares
                       # (futuro) middleware/, errors/
  Dockerfile           # multi-stage, distroless nonroot
  Makefile             # alvos locais (dev/test/build/lint/...)
```

A camada hexagonal (domain/application/infra) entra em `libs/<bounded-context>/...` (ver Fase 2 do roadmap).
`apps/api` é só **delivery + composition root** — handlers chi cabeiam casos de uso vindos de `libs/`.

## Rotas

- `GET /healthz` — liveness
- `GET /readyz` — readiness (TODO: checar DB + Kafka)

## Dev

```bash
make dev          # go run ./cmd/api
make test         # go test -race
make build        # bin/api
make help         # lista todos os alvos
```

Variáveis (com defaults sensatos pra dev local):

| Env            | Default                                                              |
|----------------|----------------------------------------------------------------------|
| `HTTP_ADDR`    | `:8080`                                                              |
| `DATABASE_URL` | `postgres://iris:iris@localhost:5432/iris?sslmode=disable`           |
| `KAFKA_BROKER` | `localhost:9092`                                                     |
| `ENV`          | `dev`                                                                |
