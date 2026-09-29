# mykomora — repo-root targets.
#
# `make dev` is the one command that has to work. Everything else either feeds
# CI or delegates to core-api/Makefile and web/package.json.

SHELL := /bin/bash

COMPOSE_FILE := deploy/docker-compose.yml
ENV_FILE := deploy/.env
COMPOSE := docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE)

.DEFAULT_GOAL := help

.PHONY: help
help: ## List the available targets
	@echo "mykomora — run 'make dev' to bring up the local stack."
	@echo
	@grep -hE '^[a-z0-9_.-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

## --- local stack -----------------------------------------------------------

$(ENV_FILE):
	@cp deploy/.env.example $(ENV_FILE)
	@echo "created $(ENV_FILE) from deploy/.env.example"

.PHONY: env
env: $(ENV_FILE) ## Create deploy/.env from the example if it is missing

# Compose mounts a named volume at web/.next so the container's build cache is
# not the host's. Docker creates a missing mount point as root, which then
# makes `next typegen` — and so `make typecheck` — fail on the host. Creating
# it first leaves it owned by whoever ran make.
web/.next:
	@mkdir -p web/.next

.PHONY: dev
dev: env web/.next ## Build and start the whole local stack
	$(COMPOSE) up --build -d
	@echo
	@echo "  web        http://localhost:$$(grep -E '^PUBLIC_PORT=' $(ENV_FILE) | cut -d= -f2)"
	@echo "  healthz    http://localhost:$$(grep -E '^PUBLIC_PORT=' $(ENV_FILE) | cut -d= -f2)/healthz"
	@echo "  ping       http://localhost:$$(grep -E '^PUBLIC_PORT=' $(ENV_FILE) | cut -d= -f2)/api/v1/ping"
	@echo "  s3         http://localhost:$$(grep -E '^S3_PORT=' $(ENV_FILE) | cut -d= -f2)"
	@echo
	@echo "  logs: make logs   stop: make down"

.PHONY: up
up: env web/.next ## Start the stack without rebuilding images
	$(COMPOSE) up -d

.PHONY: down
down: ## Stop the stack, keeping data volumes
	$(COMPOSE) down --remove-orphans

.PHONY: clean
clean: ## Stop the stack and delete its volumes, including the database
	$(COMPOSE) down --remove-orphans --volumes

.PHONY: logs
logs: ## Follow logs from every service
	$(COMPOSE) logs -f

.PHONY: ps
ps: ## Show the status of each service
	$(COMPOSE) ps

.PHONY: build
build: env web/.next ## Build every image, including the production runtime stages
	$(COMPOSE) build

## --- quality ---------------------------------------------------------------

.PHONY: generate
generate: ## Regenerate all generated code, Go and TypeScript
	$(MAKE) -C core-api generate
	cd web && npm run generate:api

.PHONY: lint
lint: ## Lint both services
	$(MAKE) -C core-api lint
	cd web && npm run lint && npm run format:check

.PHONY: fmt
fmt: ## Format both services
	$(MAKE) -C core-api fmt
	cd web && npm run format

.PHONY: test
test: ## Unit tests for both services — no Docker required
	$(MAKE) -C core-api test
	cd web && npm run test

.PHONY: test-integration
test-integration: ## Unit + integration tests — needs a Docker daemon
	$(MAKE) -C core-api test-integration
	cd web && npm run test

.PHONY: typecheck
typecheck: ## Type-check the web app
	cd web && npm run typecheck

.PHONY: check
check: lint typecheck test ## Everything CI runs, minus the Docker-bound jobs

## --- migrations ------------------------------------------------------------
# These run from the host against the published Postgres port, so the stack
# needs to be up. `make dev` already applies migrations on startup.

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	$(MAKE) -C core-api migrate-up DATABASE_URL="$(HOST_DATABASE_URL)"

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	$(MAKE) -C core-api migrate-down DATABASE_URL="$(HOST_DATABASE_URL)"

.PHONY: migrate-status
migrate-status: ## Show which migrations have been applied
	$(MAKE) -C core-api migrate-status DATABASE_URL="$(HOST_DATABASE_URL)"

.PHONY: migrate-create
migrate-create: ## Create a migration: make migrate-create name=add_items
	$(MAKE) -C core-api migrate-create name=$(name)

.PHONY: vet-sql
vet-sql: ## Lint queries, including the family-scoping rule (stack must be up)
	$(MAKE) -C core-api vet-sql DATABASE_URL="$(HOST_DATABASE_URL)"

# Same database as the stack, reached over the published port instead of the
# Compose network.
HOST_DATABASE_URL = postgres://$(shell grep -E '^POSTGRES_USER=' $(ENV_FILE) | cut -d= -f2):$(shell grep -E '^POSTGRES_PASSWORD=' $(ENV_FILE) | cut -d= -f2)@localhost:$(shell grep -E '^POSTGRES_PORT=' $(ENV_FILE) | cut -d= -f2)/$(shell grep -E '^POSTGRES_DB=' $(ENV_FILE) | cut -d= -f2)?sslmode=disable
