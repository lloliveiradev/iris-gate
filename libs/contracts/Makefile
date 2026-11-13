# libs/contracts — codegen pipeline
#
# Everything is declared to run from `libs/contracts/`.
# OpenAPI specs (`http/*.yaml`) are the single source of truth — generating types is mechanical.

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# Specs with concrete paths (skeletons without `paths` are skipped automatically).
HTTP_SPECS := http/identity.openapi.yaml http/payments.openapi.yaml http/fraud-internal.openapi.yaml

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)libs/contracts$(C_RST) — targets:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-14s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: gen
gen: gen-go gen-ts ## Generate Go + TS types

.PHONY: gen-go
gen-go: ## Generate Go types (oapi-codegen) — registered as a tool in generated/go/go.mod
	@mkdir -p generated/go/identity generated/go/payments generated/go/fraudinternal
	@# Filter out the known OpenAPI 3.1.x warning (oapi-codegen v2 does not support it natively yet).
	go -C generated/go tool oapi-codegen \
		--config ../../codegen/identity.go.yaml \
		../../http/identity.openapi.yaml \
		2> >(grep -v "OpenAPI 3.1.x specification" >&2 || true)
	go -C generated/go tool oapi-codegen \
		--config ../../codegen/payments.go.yaml \
		../../http/payments.openapi.yaml \
		2> >(grep -v "OpenAPI 3.1.x specification" >&2 || true)
	go -C generated/go tool oapi-codegen \
		--config ../../codegen/fraud-internal.go.yaml \
		../../http/fraud-internal.openapi.yaml \
		2> >(grep -v "OpenAPI 3.1.x specification" >&2 || true)
	go -C generated/go mod tidy

.PHONY: gen-ts
gen-ts: ## Generate TS types (openapi-typescript)
	@mkdir -p generated/ts
	yarn run gen

.PHONY: check
check: ## Run gen + fail if generated files drifted (CI gate)
	$(MAKE) gen
	@if ! git diff --exit-code -- generated/ ; then \
		echo ""; \
		echo "✗ generated types are out of sync with the specs."; \
		echo "  run 'make gen-contracts' at the root and commit generated/."; \
		exit 1; \
	fi
	@echo "✓ contratos em sync."

.PHONY: typecheck
typecheck: ## tsc --noEmit (libs/contracts)
	yarn run typecheck

.PHONY: clean
clean: ## Remove generated code
	rm -rf generated/go/identity generated/go/payments generated/go/fraudinternal generated/ts
