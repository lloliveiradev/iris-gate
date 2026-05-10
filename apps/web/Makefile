# apps/web — Next 15 (App Router, React 19)

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# Usa o yarn raiz (workspaces). Se rodar de fora, ainda funciona.
YARN := yarn

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Mostra esta ajuda
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)apps/web$(C_RST) — alvos:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-12s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: dev
dev: ## next dev (Turbopack, :3000)
	$(YARN) run dev

.PHONY: build
build: ## next build
	$(YARN) run build

.PHONY: start
start: ## next start (após build)
	$(YARN) run start

.PHONY: test
test: ## vitest
	$(YARN) run test

.PHONY: lint
lint: ## next lint (eslint flat config)
	$(YARN) run lint

.PHONY: typecheck
typecheck: ## tsc --noEmit
	$(YARN) run typecheck

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf .next dist coverage .turbo
