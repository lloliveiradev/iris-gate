# apps/api — Go service (chi + slog)

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO       ?= go
PKG      := ./...
GOLANGCI_CONFIG := ../../configs/go/golangci.yml
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

.PHONY: test
test: ## go test -race
	$(GO) test -race -count=1 $(PKG)

.PHONY: cover
cover: ## go test with coverage
	$(GO) test -race -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: lint
lint: ## golangci-lint with configs/go/golangci.yml
	@command -v golangci-lint >/dev/null || { echo "install: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run --config $(GOLANGCI_CONFIG) $(PKG)

.PHONY: fmt
fmt: ## gofumpt
	@command -v gofumpt >/dev/null || { echo "install: go install mvdan.cc/gofumpt@latest"; exit 1; }
	gofumpt -l -w .

.PHONY: vet
vet: ## go vet
	$(GO) vet $(PKG)

.PHONY: clean
clean: ## Remove the binary and coverage output
	rm -rf $(BIN_DIR) coverage.out

.PHONY: docker-build
docker-build: ## Build the Docker image
	docker build -t iris-gate/api:dev .
