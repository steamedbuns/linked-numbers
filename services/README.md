# services

One Go module with one `cmd/<name>` per binary:

| Binary | Port | Role |
| --- | --- | --- |
| `values-api` | 8081 | REST API; the only service that reads or writes the tables |
| `realtime-gateway` | 8082 | LISTENs on `values_changed` and pushes events over WebSocket (LN-5) |
| `explain-api` | 8083 | AI change summaries that cite history rows (LN-8) |

Every binary starts from the service template ([ADR 0004](../docs/adr/0004-go-service-template.md)):

| Package | Contents |
| --- | --- |
| `internal/service` | `Service.Main`: env config, JSON logger, `/healthz` and `/readyz`, graceful shutdown on SIGTERM |
| `internal/logging` | JSON slog logger that adds `trace_id`/`span_id` from the context |
| `internal/httpx` | Request middleware (trace context, access log, panic recovery) and RFC 9457 problem responses |
| `internal/buildinfo` | The VCS revision, logged as `version` |
| `internal/db` | Goose migrations, seed data and queries (all embedded or generated from here); `Migrate` and `Seed` ([ADR 0005](../docs/adr/0005-database-schema-and-migrations.md)) |
| `internal/db/dbtest` | Test helper: one Postgres container per test binary, a fresh migrated (`New`) or seeded (`NewSeeded`) database per test |
| `internal/store` | sqlc-generated queries and models. Don't edit; regenerate |

## Configuration

| Env var | Default | Meaning |
| --- | --- | --- |
| `HTTP_ADDR` | the service's port, e.g. `:8081` | Listen address |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `SHUTDOWN_TIMEOUT` | `10s` | How long to wait for in-flight requests after SIGTERM (at most `10s`) |
| `DATABASE_URL` | none; values-api requires it | Postgres URL, e.g. the local stack's in [.env.example](../.env.example) |
| `MIGRATE_ON_START` | `false` | Apply pending migrations before serving. For dev; Kubernetes runs them as a Job (LN-9.2) |

## Run

```bash
./dev run make up               # local Postgres
./dev run make run-values-api   # DATABASE_URL of the local stack, MIGRATE_ON_START=true
```

Then `curl localhost:8081/healthz` and `curl localhost:8081/readyz`. `/readyz` returns 503 until Postgres answers (`postgres` check) and every migration is applied (`schema` check).

## Database

The schema is in [internal/db/migrations](internal/db/migrations), as goose SQL files. To add a migration, create the next numbered file with `-- +goose Up` and `-- +goose Down` sections:

```bash
./dev run goose -dir services/internal/db/migrations -s create add_something sql
```

Against the local stack (`make up` first):

| Command | Does |
| --- | --- |
| `./dev run make db-migrate` | Apply pending migrations |
| `./dev run make db-seed` | Insert the demo data: 3 users, 12 values, 2 reports ([seed.sql](internal/db/seed.sql)). Existing rows are skipped |
| `./dev run make db-reset` | Roll back every migration (deleting all data), then migrate and seed |

Queries live in [internal/db/queries](internal/db/queries). After changing a query or a migration, regenerate `internal/store` and commit it. `make lint-sqlc` fails in CI if it is stale:

```bash
./dev run bash -c 'cd services && sqlc generate'
```

Tests that need Postgres call `dbtest.Main` from `TestMain`, then `dbtest.New(t)` (migrated), `dbtest.NewSeeded(t)` or `dbtest.NewEmpty(t)` per test. They need Docker, which the dev container and CI both have.
