ENV_FILE ?= .env
COMPOSE_FILE := deploy/docker-compose.yml
COMPOSE := docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE)
GO_MODULES := . services/api-gateway services/auth-service services/page-service services/collaboration-service services/knowledge-graph-search-service services/mws-integration-service services/file-service

.PHONY: help env bootstrap workspace-sync compose-build compose-up compose-down compose-logs compose-smoke migrate migrate-down seed-auth-demo test fmt

help:
	@echo "Available targets:"
	@echo "  env             Copy .env.example to .env if needed"
	@echo "  bootstrap       Prepare the local demo stack"
	@echo "  workspace-sync  Sync the Go workspace"
	@echo "  compose-build   Build all Docker images"
	@echo "  compose-up      Start the local stack"
	@echo "  compose-down    Stop the local stack"
	@echo "  compose-logs    Tail compose logs"
	@echo "  compose-smoke   Run the Docker Compose demo smoke validation"
	@echo "  migrate         Run all available migrations"
	@echo "  migrate-down    Roll back the latest migration where available"
	@echo "  seed-auth-demo  Seed demo auth users and memberships"
	@echo "  test            Run Go tests across the workspace"
	@echo "  fmt             Run gofmt across tracked Go files"

env:
	@if [ ! -f $(ENV_FILE) ]; then cp .env.example $(ENV_FILE); echo "Created $(ENV_FILE) from .env.example"; else echo "$(ENV_FILE) already exists"; fi

bootstrap:
	@bash scripts/bootstrap-demo.sh

workspace-sync:
	@go work sync

compose-build:
	@$(COMPOSE) build

compose-up:
	@$(COMPOSE) up -d --build

compose-down:
	@$(COMPOSE) down --remove-orphans

compose-logs:
	@$(COMPOSE) logs -f

compose-smoke:
	@bash tests/compose/demo_smoke_test.sh

migrate:
	@bash scripts/migrate.sh up

migrate-down:
	@bash scripts/migrate.sh down

seed-auth-demo:
	@bash -lc 'set -a; [ -f "$(ENV_FILE)" ] && . "$(ENV_FILE)"; set +a; cd services/auth-service && go run ./cmd/seed-demo'

test:
	@for module in $(GO_MODULES); do \
		echo "==> $$module"; \
		(cd $$module && go test ./...); \
	done

fmt:
	@go fmt ./...
	@for module in $(wordlist 2,$(words $(GO_MODULES)),$(GO_MODULES)); do \
		(cd $$module && go fmt ./...); \
	done
