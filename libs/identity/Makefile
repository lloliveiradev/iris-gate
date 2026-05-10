# libs/identity — domain + application + infra (Identity bounded context)

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO  ?= go
PKG := ./...

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Mostra esta ajuda
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)libs/identity$(C_RST) — alvos:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-12s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: test
test: ## go test -race
	$(GO) test -race -count=1 $(PKG)

.PHONY: test-int
test-int: ## go test -race com tag de integração (requer Postgres)
	$(GO) test -race -count=1 -tags=integration $(PKG)

.PHONY: cover
cover: ## go test cobertura
	$(GO) test -race -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: vet
vet: ## go vet
	$(GO) vet $(PKG)

.PHONY: lint
lint: ## golangci-lint (precisa instalado)
	@command -v golangci-lint >/dev/null || { echo "instale: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run

.PHONY: fmt
fmt: ## gofumpt
	@command -v gofumpt >/dev/null || { echo "instale: go install mvdan.cc/gofumpt@latest"; exit 1; }
	gofumpt -l -w .

.PHONY: build
build: ## go build (smoke check de compilação)
	$(GO) build ./...

.PHONY: clean
clean: ## Remove artefatos de teste
	rm -f coverage.out
