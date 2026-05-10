# apps/anti-fraud — Nest 11

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

YARN := yarn

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Mostra esta ajuda
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)apps/anti-fraud$(C_RST) — alvos:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-12s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: dev
dev: ## nest start --watch
	$(YARN) run dev

.PHONY: build
build: ## nest build
	$(YARN) run build

.PHONY: start
start: ## node dist/main.js
	$(YARN) run start

.PHONY: test
test: ## jest
	$(YARN) run test

.PHONY: lint
lint: ## eslint
	$(YARN) run lint

.PHONY: typecheck
typecheck: ## tsc --noEmit
	$(YARN) run typecheck

.PHONY: clean
clean: ## Remove build
	rm -rf dist coverage
