# core-api targets. The repo-root Makefile delegates here; run either.
#
# Dev tools (oapi-codegen, sqlc, goose, air) are pinned as `tool` directives in
# go.mod and invoked with `go tool`, so there is nothing to install first and CI
# uses byte-identical versions.

SHELL := /bin/bash
MODULE := github.com/olehmushka/mykomora/core-api
MIGRATIONS_DIR := db/migrations
GOOSE := go tool goose -dir $(MIGRATIONS_DIR)

# Local default; Compose and CI inject their own.
DATABASE_URL ?= postgres://mykomora:mykomora@localhost:5432/mykomora?sslmode=disable
export DATABASE_URL

.PHONY: help
help: ## List the available targets
	@grep -hE '^[a-z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

## --- code generation -------------------------------------------------------

.PHONY: generate
generate: generate-openapi generate-sqlc ## Regenerate all generated Go code

.PHONY: generate-openapi
generate-openapi: ## Generate server types and routes from api/openapi.yaml
	go tool oapi-codegen -config oapi-codegen.yaml api/openapi.yaml

.PHONY: generate-sqlc
generate-sqlc: ## Generate type-safe query code from db/queries
	go tool sqlc generate

## --- quality ---------------------------------------------------------------

.PHONY: fmt
fmt: ## Format and fix imports
	golangci-lint fmt ./...

.PHONY: lint
lint: ## Run the full linter set
	golangci-lint run ./...

.PHONY: vet-sql
vet-sql: ## Lint queries, including the family-scoping rule (needs a database)
	go tool sqlc vet

.PHONY: test
test: ## Unit tests only — no Docker required
	go test -race -count=1 ./...

.PHONY: test-integration
test-integration: ## Unit + integration tests — needs a Docker daemon
	go test -race -count=1 -tags=integration -timeout=600s ./...

## --- build and run ---------------------------------------------------------

.PHONY: build
build: ## Build the API binary into bin/
	CGO_ENABLED=0 go build -trimpath -o bin/api ./cmd/api

.PHONY: run
run: ## Run with live reload against a local database
	go tool air -c .air.toml

## --- migrations ------------------------------------------------------------

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	$(GOOSE) postgres "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	$(GOOSE) postgres "$(DATABASE_URL)" down

.PHONY: migrate-reset
migrate-reset: ## Roll every migration back to base
	$(GOOSE) postgres "$(DATABASE_URL)" down-to 0

.PHONY: migrate-status
migrate-status: ## Show which migrations have been applied
	$(GOOSE) postgres "$(DATABASE_URL)" status

.PHONY: migrate-create
migrate-create: ## Create a new SQL migration: make migrate-create name=add_items
	@test -n "$(name)" || { echo "usage: make migrate-create name=<snake_case_name>"; exit 1; }
	$(GOOSE) create $(name) sql
