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
| `internal/db` | Goose migrations (embedded) and `Migrate` ([ADR 0005](../docs/adr/0005-database-schema-and-migrations.md)) |
| `internal/db/dbtest` | Test helper: one Postgres container per test binary, a fresh migrated database per test |

## Configuration

| Env var | Default | Meaning |
| --- | --- | --- |
| `HTTP_ADDR` | the service's port, e.g. `:8081` | Listen address |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `SHUTDOWN_TIMEOUT` | `10s` | How long to wait for in-flight requests after SIGTERM (at most `10s`) |

## Run

```bash
./dev run bash -c 'cd services && go run ./cmd/values-api'
```

Then `curl localhost:8081/healthz` and `curl localhost:8081/readyz`.

## Database

The schema is in [internal/db/migrations](internal/db/migrations), as goose SQL files. To add a migration, create the next numbered file with `-- +goose Up` and `-- +goose Down` sections:

```bash
./dev run goose -dir services/internal/db/migrations -s create add_something sql
```

Apply migrations to the local stack (`make up` first) with `./dev run make db-migrate`. Tests that need Postgres call `dbtest.Main` from `TestMain` and `dbtest.New(t)` per test. They need Docker, which the dev container and CI both have.
