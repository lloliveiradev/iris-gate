# infra/docker — local stack (Postgres + Kafka)

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

COMPOSE      := docker compose
ENV_FILE     := .env
ENV_EXAMPLE  := .env.example

# Read POSTGRES_USER/DB from .env (or the defaults) without sourcing the file.
PG_USER ?= $(shell [ -f $(ENV_FILE) ] && grep -E '^POSTGRES_USER=' $(ENV_FILE) | cut -d= -f2 || echo iris)
PG_DB   ?= $(shell [ -f $(ENV_FILE) ] && grep -E '^POSTGRES_DB=' $(ENV_FILE) | cut -d= -f2 || echo iris)

# Broker address for commands that run inside the Kafka container.
# This is the in-network PLAINTEXT listener, not the host port (KAFKA_PORT).
KAFKA_BOOTSTRAP ?= localhost:9092

C_RST := \033[0m
C_BLD := \033[1m
C_CYN := \033[36m

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(C_BLD)infra/docker$(C_RST) — targets:\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(C_CYN)%-16s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

$(ENV_FILE):
	@cp $(ENV_EXAMPLE) $(ENV_FILE) && echo "✓ created $(ENV_FILE) from $(ENV_EXAMPLE)"

.PHONY: up
up: $(ENV_FILE) ## Start the stack in the background and wait until healthy
	$(COMPOSE) up -d --wait

.PHONY: down
down: ## Stop containers (keep volumes)
	$(COMPOSE) down

.PHONY: restart
restart: down up ## Restart the stack

.PHONY: nuke
nuke: ## Stop containers and DELETE volumes (destructive)
	@printf "This deletes ALL data (postgres+kafka). Continue? [y/N] "; \
	read ans; [ "$$ans" = "y" ] || { echo "aborted"; exit 1; }
	$(COMPOSE) down -v

.PHONY: ps
ps: ## List container status
	$(COMPOSE) ps

.PHONY: logs
logs: ## Tail logs (Ctrl-C to exit)
	$(COMPOSE) logs -f --tail=100

.PHONY: pull
pull: ## Pull the newest images
	$(COMPOSE) pull

##@ Postgres

.PHONY: psql
psql: ## Open an interactive psql in the container
	$(COMPOSE) exec -it postgres psql -U $(PG_USER) -d $(PG_DB)

.PHONY: pg-dump
pg-dump: ## Dump the schema (stdout)
	$(COMPOSE) exec -T postgres pg_dump -U $(PG_USER) -s $(PG_DB)

##@ Kafka

.PHONY: kafka-topics
kafka-topics: ## List topics
	$(COMPOSE) exec -T kafka /opt/kafka/bin/kafka-topics.sh \
		--bootstrap-server $(KAFKA_BOOTSTRAP) --list

.PHONY: kafka-shell
kafka-shell: ## Open a shell in the Kafka container
	$(COMPOSE) exec -it kafka /bin/bash

# Usage: make kafka-create-topic NAME=foo PARTITIONS=3
.PHONY: kafka-create-topic
kafka-create-topic: ## Create a topic (NAME=... [PARTITIONS=1] [RF=1])
	@[ -n "$(NAME)" ] || { echo "usage: make kafka-create-topic NAME=<topic> [PARTITIONS=3] [RF=1]"; exit 1; }
	$(COMPOSE) exec -T kafka /opt/kafka/bin/kafka-topics.sh \
		--bootstrap-server $(KAFKA_BOOTSTRAP) \
		--create --if-not-exists \
		--topic $(NAME) \
		--partitions $(or $(PARTITIONS),1) \
		--replication-factor $(or $(RF),1)
