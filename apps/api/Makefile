# apps/api — Go service (chi + slog)

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO       ?= go
PKG      := ./...
GOLANGCI_CONFIG := ../../configs/go/golangci.yml
TOOLS_BIN := $(abspath ../../.tools/bin)
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null || echo $(TOOLS_BIN)/golangci-lint)
GOFUMPT       := $(shell command -v gofumpt 2>/dev/null || echo $(TOOLS_BIN)/gofumpt)
BIN_DIR  := bin
APP      := api

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)apps/api$(C_RST) — targets:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-12s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: dev
dev: ## Run the server in the foreground
	$(GO) run ./cmd/api

.PHONY: build
build: ## Build bin/api
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(APP) ./cmd/api

.PHONY: migrate
migrate: ## Apply the identity and payments migrations as iris_migrator (IRIS_MIGRATOR_DATABASE_URL)
	$(GO) run ./cmd/migrate

.PHONY: test
test: ## go test -race
	$(GO) test -race -count=1 $(PKG)

# The role tests read infra/docker/.env from the environment and skip without it:
#   set -a && . ../../infra/docker/.env && set +a && make test-int
.PHONY: test-int
test-int: ## go test -race with the integration tag (needs the migrated Compose Postgres)
	$(GO) test -race -count=1 -tags=integration $(PKG)

.PHONY: cover
cover: ## go test with coverage
	$(GO) test -race -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: lint
lint: ## golangci-lint with configs/go/golangci.yml
	@[ -x "$(GOLANGCI_LINT)" ] || { echo "golangci-lint not found. Run: make tools"; exit 1; }
	"$(GOLANGCI_LINT)" run --config $(GOLANGCI_CONFIG) $(PKG)

.PHONY: fmt
fmt: ## gofumpt
	@[ -x "$(GOFUMPT)" ] || { echo "gofumpt not found. Run: make tools"; exit 1; }
	"$(GOFUMPT)" -l -w .

.PHONY: vet
vet: ## go vet
	$(GO) vet $(PKG)

.PHONY: clean
clean: ## Remove the binary and coverage output
	rm -rf $(BIN_DIR) coverage.out

.PHONY: docker-build
docker-build: ## Build the Docker image (repo root context)
	docker build -f Dockerfile -t iris-gate/api:dev ../..
