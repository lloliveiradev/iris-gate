# libs/identity — domain + application + infra (Identity bounded context)

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO  ?= go
PKG := ./...
GOLANGCI_CONFIG := ../../configs/go/golangci.yml

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)libs/identity$(C_RST) — targets:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-12s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: test
test: ## go test -race
	$(GO) test -race -count=1 $(PKG)

.PHONY: test-int
test-int: ## go test -race with the integration tag (needs Postgres)
	$(GO) test -race -count=1 -tags=integration $(PKG)

.PHONY: cover
cover: ## go test with coverage
	$(GO) test -race -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: vet
vet: ## go vet
	$(GO) vet $(PKG)

.PHONY: lint
lint: ## golangci-lint with configs/go/golangci.yml
	@command -v golangci-lint >/dev/null || { echo "install: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run --config $(GOLANGCI_CONFIG) $(PKG)

.PHONY: fmt
fmt: ## gofumpt
	@command -v gofumpt >/dev/null || { echo "install: go install mvdan.cc/gofumpt@latest"; exit 1; }
	gofumpt -l -w .

.PHONY: build
build: ## go build (compile smoke check)
	$(GO) build ./...

.PHONY: clean
clean: ## Remove test artifacts
	rm -f coverage.out
