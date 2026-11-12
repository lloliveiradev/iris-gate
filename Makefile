# iris-gate — root Makefile
#
# Conventions:
#   - Every public target has a `## comment`, shown by `make help`.
#   - Fan-out targets delegate to each package Makefile with `$(MAKE) -C <dir>`
#     and skip packages that do not define the target.
#   - To add a package, list its directory in PACKAGES (and in DEV_APPS if it
#     runs on the host during development).

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# Packages with their own Makefile.
PACKAGES := apps/api apps/web apps/anti-fraud libs/identity libs/payments libs/fraud libs/sdk-web
# Processes that run on the host during development. apps/anti-fraud runs in Compose.
DEV_APPS := apps/api apps/web
INFRA    := infra/docker

# Colors
C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m
C_DIM := \033[2m

##@ General

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage: $(C_BLD)make$(C_RST) $(C_CYN)<target>$(C_RST)\n"} \
		/^[a-zA-Z_0-9\/-]+:.*?##/ { printf "  $(C_CYN)%-22s$(C_RST) %s\n", $$1, $$2 } \
		/^##@/ { printf "\n$(C_BLD)%s$(C_RST)\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Bootstrap

.PHONY: bootstrap
bootstrap: install up ## Install dependencies and start the local stack (first run)
	@echo "✓ ready. 'make ps' lists containers, 'make logs' follows them, 'make dev' starts the API and the dashboard."

.PHONY: install
install: ## Install Node dependencies (Yarn workspaces)
	@command -v yarn >/dev/null || { echo "yarn not found. Run: corepack enable"; exit 1; }
	yarn install

.PHONY: install-hooks
install-hooks: ## Install git hooks (lefthook)
	@command -v lefthook >/dev/null || { echo "lefthook not found. brew install lefthook  or  go install github.com/evilmartians/lefthook@latest"; exit 1; }
	lefthook install

.PHONY: uninstall-hooks
uninstall-hooks: ## Remove git hooks
	@command -v lefthook >/dev/null && lefthook uninstall || true

##@ Contracts (libs/contracts)

.PHONY: gen-contracts
gen-contracts: ## Generate Go and TypeScript types from the OpenAPI specs
	@$(MAKE) -C libs/contracts gen

.PHONY: gen-contracts-check
gen-contracts-check: ## Generate and fail on drift (CI gate)
	@$(MAKE) -C libs/contracts check

##@ Infra (delegates to infra/docker)

.PHONY: up down restart ps logs
up: ## Start the local stack (Postgres, Kafka, Karapace, anti-fraud)
	@$(MAKE) -C $(INFRA) up
down: ## Stop the local stack (keeps volumes)
	@$(MAKE) -C $(INFRA) down
restart: ## Restart the local stack
	@$(MAKE) -C $(INFRA) restart
ps: ## List stack containers
	@$(MAKE) -C $(INFRA) ps
logs: ## Follow stack logs (Ctrl-C to stop)
	@$(MAKE) -C $(INFRA) logs

.PHONY: nuke
nuke: ## Stop the stack and DELETE its volumes (destructive)
	@$(MAKE) -C $(INFRA) nuke

.PHONY: psql kafka-topics
psql: ## Open psql in the Postgres container
	@$(MAKE) -C $(INFRA) psql
kafka-topics: ## List Kafka topics
	@$(MAKE) -C $(INFRA) kafka-topics

##@ Development

.PHONY: dev
dev: up ## Start the stack, then run the API (:8080) and the dashboard (:3000) on the host
	@trap 'kill 0' INT TERM; \
	for d in $(DEV_APPS); do \
		echo "▸ $$d :: dev"; \
		$(MAKE) -C $$d dev & \
	done; \
	wait

# Operator CLI for merchants and API keys (replaces the closed bootstrap routes).
# Example: make irisctl ARGS="issue-api-key --merchant-id <uuid>"
# Needs the stack up (make up) and the API migrated at least once.
.PHONY: irisctl
irisctl: ## Run the operator CLI: make irisctl ARGS="create-merchant --legal-name ..."
	@cd apps/api && go run ./cmd/irisctl $(ARGS)

##@ Packages

# `make app-<name> CMD=<target>` runs one target in apps/<name>.
# Example: make app-api CMD=test  ->  $(MAKE) -C apps/api test
.PHONY: app-%
app-%: ## Run a target in apps/<name>: make app-api CMD=test
	@dir=apps/$* ; \
	[ -f $$dir/Makefile ] || { echo "$$dir/Makefile does not exist"; exit 1; } ; \
	$(MAKE) -C $$dir $(CMD)

.PHONY: test lint build clean
test: ## Run 'test' in every package that defines it
	@$(MAKE) _foreach TARGET=test
lint: ## Run 'lint' in every package that defines it
	@$(MAKE) _foreach TARGET=lint
build: ## Run 'build' in every package that defines it
	@$(MAKE) _foreach TARGET=build
clean: ## Remove node_modules, local caches and build artifacts
	@$(MAKE) _foreach TARGET=clean
	@echo "▸ workspace :: node_modules + caches"
	rm -rf \
		node_modules \
		.turbo \
		.cache \
		.yarn/cache \
		.yarn/unplugged \
		.yarn/install-state.gz
	find apps libs -type d \( \
		-name node_modules -o -name .next -o -name dist -o \
		-name .turbo -o -name .cache -o -name coverage \
	\) -prune -exec rm -rf {} +
	find apps libs -type f \( -name '*.tsbuildinfo' -o -name '.eslintcache' \) -delete

# Internal helper: runs TARGET in every package whose Makefile defines it.
.PHONY: _foreach
_foreach:
	@set -e; for d in $(PACKAGES); do \
		if [ ! -f $$d/Makefile ]; then \
			echo "skip $$d (no Makefile)"; \
		elif ! $(MAKE) -C $$d -n $(TARGET) >/dev/null 2>&1; then \
			echo "skip $$d (no '$(TARGET)' target)"; \
		else \
			echo "▸ $$d :: $(TARGET)"; \
			$(MAKE) -C $$d $(TARGET); \
		fi; \
	done
