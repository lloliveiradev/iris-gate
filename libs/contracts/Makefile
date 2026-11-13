# libs/contracts — codegen pipeline
#
# Tudo é declarado pra rodar a partir do diretório `libs/contracts/`.
# As specs OpenAPI (`http/*.yaml`) são single source of truth — gerar tipos é mecânico.

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# Specs com paths reais (skeletons sem `paths` são pulados automaticamente).
HTTP_SPECS := http/identity.openapi.yaml http/payments.openapi.yaml http/fraud-internal.openapi.yaml

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Mostra esta ajuda
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)libs/contracts$(C_RST) — alvos:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-14s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: gen
gen: gen-go gen-ts ## Gera tipos Go + TS

.PHONY: gen-go
gen-go: ## Gera tipos Go (oapi-codegen) — registrado como tool em generated/go/go.mod
	@mkdir -p generated/go/identity generated/go/payments generated/go/fraudinternal
	@# Filtra o WARNING conhecido de OpenAPI 3.1.x (oapi-codegen v2 ainda não suporta nativamente).
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
gen-ts: ## Gera tipos TS (openapi-typescript)
	@mkdir -p generated/ts
	yarn run gen

.PHONY: check
check: ## Roda gen + falha se houver drift (CI gate)
	$(MAKE) gen
	@if ! git diff --exit-code -- generated/ ; then \
		echo ""; \
		echo "✗ tipos gerados estão fora de sync com as specs."; \
		echo "  rode 'make gen-contracts' na raiz e comite generated/."; \
		exit 1; \
	fi
	@echo "✓ contratos em sync."

.PHONY: typecheck
typecheck: ## tsc --noEmit (libs/contracts)
	yarn run typecheck

.PHONY: clean
clean: ## Limpa código gerado
	rm -rf generated/go/identity generated/go/payments generated/go/fraudinternal generated/ts
