# apps/api — Go service (chi + slog)

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO       ?= go
PKG      := ./...
BIN_DIR  := bin
APP      := api

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Mostra esta ajuda
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)apps/api$(C_RST) — alvos:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-12s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: dev
dev: ## Roda servidor em foreground
	$(GO) run ./cmd/api

.PHONY: build
build: ## Compila bin/api
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(APP) ./cmd/api

.PHONY: test
test: ## go test -race
	$(GO) test -race -count=1 $(PKG)

.PHONY: cover
cover: ## go test com cobertura
	$(GO) test -race -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: lint
lint: ## golangci-lint (precisa estar instalado)
	@command -v golangci-lint >/dev/null || { echo "instale: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run

.PHONY: fmt
fmt: ## gofumpt
	@command -v gofumpt >/dev/null || { echo "instale: go install mvdan.cc/gofumpt@latest"; exit 1; }
	gofumpt -l -w .

.PHONY: vet
vet: ## go vet
	$(GO) vet $(PKG)

.PHONY: clean
clean: ## Remove binário e cobertura
	rm -rf $(BIN_DIR) coverage.out

.PHONY: docker-build
docker-build: ## Constrói imagem Docker
	docker build -t iris-gate/api:dev .
