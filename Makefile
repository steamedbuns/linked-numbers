# Run from the repo root, on the host or with `./dev run make <target>`.
COMPOSE := docker compose -f deploy/compose/compose.yaml --env-file $(or $(wildcard .env),/dev/null)

.DEFAULT_GOAL := help
.PHONY: help up down clean ps logs psql check lint-go test-go lint-web test-web lint-api

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-9s %s\n", $$1, $$2}'

up: ## Start Postgres and Jaeger and wait until Postgres is healthy
	$(COMPOSE) up -d --wait

down: ## Stop the stack (keeps the database volume)
	$(COMPOSE) down

clean: ## Stop the stack and delete the database volume
	$(COMPOSE) down -v

ps: ## Show stack status
	$(COMPOSE) ps

logs: ## Follow stack logs
	$(COMPOSE) logs -f

psql: ## Open psql on the local database
	$(COMPOSE) exec postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

# ---- Checks: CI runs these same targets (.github/workflows/ci.yml) ----------

check: lint-go test-go lint-web test-web lint-api ## Run every CI check

lint-go: ## Lint and format-check Go; check go.mod is tidy
	cd services && golangci-lint run ./... && go mod tidy -diff

test-go: ## Run Go unit tests with the race detector
	cd services && go test -race ./...

lint-web: ## Format-check and analyze Dart
	cd web && dart pub get --enforce-lockfile && dart format --output=none --set-exit-if-changed . && dart analyze --fatal-infos

test-web: ## Run Dart unit tests
	cd web && dart test

lint-api: ## Lint the OpenAPI spec
	redocly lint api/openapi.yaml
