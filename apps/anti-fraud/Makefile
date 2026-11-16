# apps/anti-fraud — NestJS 12 worker (Prisma 7, KafkaJS)

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

YARN := yarn

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)apps/anti-fraud$(C_RST) — targets:\n"} \
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

# A disposable database for the integration specs (for example `antifraud_test`
# on the Compose Postgres), never the dev one: the specs write rows. The recipe
# reads it from the shell so the URL and its password are not echoed.
export ANTIFRAUD_TEST_DATABASE_URL

.PHONY: test-int
test-int: ## migrate and run the integration specs (needs Postgres, ANTIFRAUD_TEST_DATABASE_URL)
	@[ -n "$${ANTIFRAUD_TEST_DATABASE_URL:-}" ] || { echo "ANTIFRAUD_TEST_DATABASE_URL is not set"; exit 1; }
	ANTIFRAUD_MIGRATOR_DATABASE_URL="$$ANTIFRAUD_TEST_DATABASE_URL" $(YARN) run prisma:migrate:deploy
	$(YARN) run test --testPathPatterns '\.integration\.spec\.ts$$'

.PHONY: lint
lint: ## eslint
	$(YARN) run lint

.PHONY: typecheck
typecheck: ## tsc --noEmit
	$(YARN) run typecheck

.PHONY: prisma-generate
prisma-generate: ## prisma generate (typed client)
	$(YARN) run prisma:generate

.PHONY: prisma-migrate
prisma-migrate: ## prisma migrate dev (local)
	$(YARN) run prisma:migrate:dev

# Runs as antifraud_migrator (ANTIFRAUD_MIGRATOR_DATABASE_URL). Its default
# privileges also grant antifraud_app DML on Prisma's own _prisma_migrations
# table; the REVOKE takes that back so the app cannot mark a migration applied.
.PHONY: prisma-deploy
prisma-deploy: ## prisma migrate deploy as antifraud_migrator, then lock _prisma_migrations
	$(YARN) run prisma:migrate:deploy
	echo 'REVOKE ALL ON TABLE "_prisma_migrations" FROM antifraud_app;' | $(YARN) prisma db execute --stdin

.PHONY: clean
clean: ## Remove node_modules, caches and build output
	rm -rf node_modules dist coverage .turbo .cache
	rm -f tsconfig.tsbuildinfo
