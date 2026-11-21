# libs/payments — domain + application + infra (Payments bounded context)

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO  ?= go
PKG := ./...
GOLANGCI_CONFIG := ../../configs/go/golangci.yml
TOOLS_BIN := $(abspath ../../.tools/bin)
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null || echo $(TOOLS_BIN)/golangci-lint)
GOFUMPT       := $(shell command -v gofumpt 2>/dev/null || echo $(TOOLS_BIN)/gofumpt)

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)libs/payments$(C_RST) — targets:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-12s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: test
test: ## go test -race
	$(GO) test -race -count=1 $(PKG)

.PHONY: test-int
# The row-level security tests (TestIntegration_RLS_*) connect as iris_app and
# fail when IRIS_APP_DATABASE_URL is unset. Where the database roles do not
# exist (CI go-libs: bootstrap superuser only), opt out explicitly with
# SKIP_RLS=1; CI api-int runs them with the roles.
RLS_TEST_PATTERN := ^TestIntegration_RLS_
ifeq ($(SKIP_RLS),1)
TEST_INT_SKIP := -skip '$(RLS_TEST_PATTERN)'
endif
test-int: ## go test -race with the integration tag (needs Postgres, DATABASE_URL, IRIS_APP_DATABASE_URL; SKIP_RLS=1 skips the RLS tests)
	$(GO) test -race -count=1 -tags=integration $(TEST_INT_SKIP) $(PKG)

.PHONY: cover
cover: ## go test with coverage
	$(GO) test -race -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: vet
vet: ## go vet
	$(GO) vet $(PKG)

.PHONY: lint
lint: ## golangci-lint with configs/go/golangci.yml
	@[ -x "$(GOLANGCI_LINT)" ] || { echo "golangci-lint not found. Run: make tools"; exit 1; }
	"$(GOLANGCI_LINT)" run --config $(GOLANGCI_CONFIG) $(PKG)

.PHONY: fmt
fmt: ## gofumpt
	@[ -x "$(GOFUMPT)" ] || { echo "gofumpt not found. Run: make tools"; exit 1; }
	"$(GOFUMPT)" -l -w .

.PHONY: build
build: ## go build (compile smoke check)
	$(GO) build ./...

.PHONY: clean
clean: ## Remove test artifacts
	rm -f coverage.out
