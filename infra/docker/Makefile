# infra/docker — stack local (Postgres + Kafka)

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

COMPOSE      := docker compose
ENV_FILE     := .env
ENV_EXAMPLE  := .env.example

# Lê POSTGRES_USER/DB do .env (ou usa defaults) sem precisar 'source'.
PG_USER ?= $(shell [ -f $(ENV_FILE) ] && grep -E '^POSTGRES_USER=' $(ENV_FILE) | cut -d= -f2 || echo iris)
PG_DB   ?= $(shell [ -f $(ENV_FILE) ] && grep -E '^POSTGRES_DB=' $(ENV_FILE) | cut -d= -f2 || echo iris)

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Mostra esta ajuda
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)infra/docker$(C_RST) — alvos:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-16s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

$(ENV_FILE):
	@cp $(ENV_EXAMPLE) $(ENV_FILE) && echo "✓ criado $(ENV_FILE) a partir de $(ENV_EXAMPLE)"

.PHONY: up
up: $(ENV_FILE) ## Sobe stack em background e espera ficar saudável
	$(COMPOSE) up -d --wait

.PHONY: down
down: ## Para containers (mantém volumes)
	$(COMPOSE) down

.PHONY: restart
restart: down up ## Reinicia stack

.PHONY: nuke
nuke: ## Para containers e APAGA volumes (destrutivo)
	@printf "Vai apagar TODOS os dados (postgres+kafka). Continuar? [y/N] "; \
	read ans; [ "$$ans" = "y" ] || { echo "abortado"; exit 1; }
	$(COMPOSE) down -v

.PHONY: ps
ps: ## Lista status dos containers
	$(COMPOSE) ps

.PHONY: logs
logs: ## Tail de logs (Ctrl-C pra sair)
	$(COMPOSE) logs -f --tail=100

.PHONY: pull
pull: ## Baixa imagens mais recentes
	$(COMPOSE) pull

##@ Postgres

.PHONY: psql
psql: ## Abre psql interativo no container
	$(COMPOSE) exec -it postgres psql -U $(PG_USER) -d $(PG_DB)

.PHONY: pg-dump
pg-dump: ## Dump do schema (stdout)
	$(COMPOSE) exec -T postgres pg_dump -U $(PG_USER) -s $(PG_DB)

##@ Kafka

.PHONY: kafka-topics
kafka-topics: ## Lista tópicos
	$(COMPOSE) exec -T kafka /opt/kafka/bin/kafka-topics.sh \
		--bootstrap-server localhost:9092 --list

.PHONY: kafka-shell
kafka-shell: ## Abre shell no container do kafka
	$(COMPOSE) exec -it kafka /bin/bash

# Uso: make kafka-create-topic NAME=foo PARTITIONS=3
.PHONY: kafka-create-topic
kafka-create-topic: ## Cria tópico (NAME=... [PARTITIONS=1] [RF=1])
	@[ -n "$(NAME)" ] || { echo "uso: make kafka-create-topic NAME=<topico> [PARTITIONS=3] [RF=1]"; exit 1; }
	$(COMPOSE) exec -T kafka /opt/kafka/bin/kafka-topics.sh \
		--bootstrap-server localhost:9092 \
		--create --if-not-exists \
		--topic $(NAME) \
		--partitions $(or $(PARTITIONS),1) \
		--replication-factor $(or $(RF),1)
