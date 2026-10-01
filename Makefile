# Run from the repo root, on the host or with `./dev run make <target>`.
COMPOSE := docker compose -f deploy/compose/compose.yaml --env-file $(or $(wildcard .env),/dev/null)
# The local stack's database (same default as .env.example); override to target another.
DATABASE_URL ?= postgres://linked_numbers:linked_numbers@localhost:5432/linked_numbers?sslmode=disable
MIGRATIONS := services/internal/db/migrations
GOOSE = goose -dir $(MIGRATIONS) postgres "$(DATABASE_URL)"

.DEFAULT_GOAL := help
.PHONY: help up down clean ps logs psql db-migrate db-seed db-reset run-values-api serve-web check lint-go lint-sqlc test-go lint-web test-web lint-api

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-15s %s\n", $$1, $$2}'

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

db-migrate: ## Apply pending migrations to DATABASE_URL (default: the local stack)
	$(GOOSE) up

db-seed: ## Insert the demo data into DATABASE_URL (skips rows that exist)
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -1 -q -f services/internal/db/seed.sql

db-reset: ## Roll back every migration in DATABASE_URL (deletes all data), then migrate and seed
	$(GOOSE) reset
	$(MAKE) db-migrate db-seed

run-values-api: ## Run values-api on :8081 against DATABASE_URL, migrating on start
	cd services && DATABASE_URL="$(DATABASE_URL)" MIGRATE_ON_START=true go run ./cmd/values-api

serve-web: ## Serve the web app at http://localhost:8080
	cd web && webdev serve web:8080

# ---- Checks: CI runs these same targets (.github/workflows/ci.yml) ----------

check: lint-go lint-sqlc test-go lint-web test-web lint-api ## Run every CI check

lint-go: ## Lint and format-check Go; check go.mod is tidy
	cd services && golangci-lint run ./... && go mod tidy -diff

lint-sqlc: ## Check the committed sqlc output (services/internal/store) is up to date
	scripts/ci/check-sqlc.sh

test-go: ## Run Go tests with the race detector (needs Docker for Postgres)
	cd services && go test -race ./...

# Generated *.g.dart files are git-ignored, so build them before analyzing.
lint-web: ## Generate code, then format-check and analyze Dart
	cd web && dart pub get --enforce-lockfile \
	  && dart run build_runner build \
	  && find lib test web -name '*.dart' ! -name '*.g.dart' -print0 | xargs -0 dart format --output=none --set-exit-if-changed \
	  && dart analyze --fatal-infos

# --no-symlink: the test runner won't serve symlinks that point outside its
# precompiled directory, and build_runner test symlinks by default.
test-web: ## Run Dart tests in Chrome via build_runner
	cd web && dart run build_runner test --no-symlink -- -p chrome

lint-api: ## Lint the OpenAPI spec
	redocly lint api/openapi.yaml
