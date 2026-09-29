# Run from the repo root, on the host or with `./dev run make <target>`.
COMPOSE := docker compose -f deploy/compose/compose.yaml --env-file $(or $(wildcard .env),/dev/null)

.DEFAULT_GOAL := help
.PHONY: help up down clean ps logs psql

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-8s %s\n", $$1, $$2}'

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
