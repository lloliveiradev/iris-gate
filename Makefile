# iris-gate — root Makefile
#
# Convenções:
#   - Todo target público tem `## comentário` (mostrado em `make help`).
#   - Targets de fan-out delegam para os Makefiles dos "galhos" via `$(MAKE) -C <dir>`.
#   - Galhos atuais: infra/docker, apps/api, apps/web, apps/anti-fraud (conforme forem scaffoldados).
#   - Adicionar novo galho: incluir o diretório em APPS ou INFRA abaixo.

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# Galhos (cada um deve ter seu próprio Makefile)
APPS  := apps/api apps/web apps/anti-fraud libs/identity libs/payments libs/fraud
INFRA := infra/docker

# Cores
C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m
C_DIM := \033[2m

##@ Geral

.PHONY: help
help: ## Mostra esta ajuda
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage: $(C_BLD)make$(C_RST) $(C_CYN)<target>$(C_RST)\n"} \
		/^[a-zA-Z_0-9\/-]+:.*?##/ { printf "  $(C_CYN)%-22s$(C_RST) %s\n", $$1, $$2 } \
		/^##@/ { printf "\n$(C_BLD)%s$(C_RST)\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Bootstrap

.PHONY: bootstrap
bootstrap: install up ## Instala deps e sobe a infra local (one-shot pra dev novo)
	@echo "✓ pronto. Use 'make ps' pra ver containers, 'make logs' pra acompanhar."

.PHONY: install
install: ## Instala deps Node (yarn workspaces)
	@command -v yarn >/dev/null || { echo "yarn não encontrado. Instale: corepack enable"; exit 1; }
	yarn install

.PHONY: install-hooks
install-hooks: ## Instala git hooks (lefthook)
	@command -v lefthook >/dev/null || { echo "lefthook não instalado. brew install lefthook  ou  go install github.com/evilmartians/lefthook@latest"; exit 1; }
	lefthook install

##@ Contratos (libs/contracts)

.PHONY: gen-contracts
gen-contracts: ## Gera tipos Go + TS a partir das specs OpenAPI
	@$(MAKE) -C libs/contracts gen

.PHONY: gen-contracts-check
gen-contracts-check: ## Gera e falha se houver drift (CI gate)
	@$(MAKE) -C libs/contracts check

.PHONY: uninstall-hooks
uninstall-hooks: ## Remove git hooks
	@command -v lefthook >/dev/null && lefthook uninstall || true

##@ Infra (delega → infra/docker)

.PHONY: up down restart ps logs
up: ## Sobe stack local (postgres + kafka)
	@$(MAKE) -C $(INFRA) up
down: ## Para stack local (mantém volumes)
	@$(MAKE) -C $(INFRA) down
restart: ## Reinicia stack local
	@$(MAKE) -C $(INFRA) restart
ps: ## Lista containers da stack
	@$(MAKE) -C $(INFRA) ps
logs: ## Acompanha logs (Ctrl-C pra sair)
	@$(MAKE) -C $(INFRA) logs

.PHONY: nuke
nuke: ## Para stack e APAGA volumes (destrutivo)
	@$(MAKE) -C $(INFRA) nuke

.PHONY: psql kafka-topics
psql: ## Abre psql no postgres
	@$(MAKE) -C $(INFRA) psql
kafka-topics: ## Lista tópicos do kafka
	@$(MAKE) -C $(INFRA) kafka-topics

##@ Fan-out por app

# Padrão: `make app-<dir> CMD=<target>` chama esse target no Makefile do app.
# Ex: make app-api CMD=test  →  $(MAKE) -C apps/api test
.PHONY: app-%
app-%: ## Roda um target em apps/<nome>: make app-api CMD=test
	@dir=apps/$* ; \
	[ -f $$dir/Makefile ] || { echo "$$dir/Makefile não existe ainda"; exit 1; } ; \
	$(MAKE) -C $$dir $(CMD)

# Targets agregados — só executam onde existir Makefile (apps ainda podem estar vazios).
.PHONY: dev test lint build clean
dev: ## Roda 'dev' em todos os apps que tiverem Makefile (paralelo)
	@$(MAKE) _foreach TARGET=dev PARALLEL=1
test: ## Roda 'test' em todos os apps que tiverem Makefile
	@$(MAKE) _foreach TARGET=test
lint: ## Roda 'lint' em todos os apps que tiverem Makefile
	@$(MAKE) _foreach TARGET=lint
build: ## Roda 'build' em todos os apps que tiverem Makefile
	@$(MAKE) _foreach TARGET=build
clean: ## Roda 'clean' em todos os apps que tiverem Makefile
	@$(MAKE) _foreach TARGET=clean

# Helper interno: itera APPS e chama TARGET onde houver Makefile.
.PHONY: _foreach
_foreach:
	@set -e; for d in $(APPS); do \
		if [ -f $$d/Makefile ]; then \
			echo "▸ $$d :: $(TARGET)"; \
			if [ "$(PARALLEL)" = "1" ]; then \
				$(MAKE) -C $$d $(TARGET) & \
			else \
				$(MAKE) -C $$d $(TARGET); \
			fi; \
		else \
			echo "skip $$d (sem Makefile)"; \
		fi; \
	done; \
	if [ "$(PARALLEL)" = "1" ]; then wait; fi
