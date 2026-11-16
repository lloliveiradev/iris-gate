# syntax=docker/dockerfile:1.7
#
# Production API image.
# Build context must be the repo root so local module replacements
# (libs/identity, libs/payments, libs/contracts/generated/go) resolve.
# The ignore rules for that context live in Dockerfile.dockerignore, next to this file.

FROM golang:1.27.4-alpine AS build
WORKDIR /src

# --- Layer 1: workspace and module manifests (cached unless go.mod/go.sum change) ---
COPY go.work ./
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY libs/identity/go.mod libs/identity/go.sum ./libs/identity/
COPY libs/payments/go.mod libs/payments/go.sum ./libs/payments/
COPY libs/contracts/generated/go/go.mod libs/contracts/generated/go/go.sum ./libs/contracts/generated/go/

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -C apps/api

# --- Layer 2: Go sources and static binary build ---
COPY libs/contracts/generated/go ./libs/contracts/generated/go
COPY libs/identity ./libs/identity
COPY libs/payments ./libs/payments
COPY apps/api ./apps/api

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -C apps/api -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
