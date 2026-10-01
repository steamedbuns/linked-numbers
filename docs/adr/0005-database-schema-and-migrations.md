# 0005. Database schema and migrations

- Status: Accepted
- Date: 2026-09-30

## Context

LN-1.5 asks for goose migrations that create all six tables, a seed with 12 values and 2 reports, and a CI check that the committed sqlc output is fresh. The spec's data model listed five tables. Every LN-2 and LN-3 story builds on this schema, and several of their acceptance criteria (append-only history, one history row per edit, blocked deletes, sums with explicit inputs) are cheapest to guarantee in the database itself.

## Decision

- **Six tables.** The sixth is `users` (`id` like `u_ana`, `display_name`). It backs the user picker, and `updated_by`, `changed_by` and `created_by` reference it, so an unknown `X-User-Id` can't be written. The spec's data model now lists it.
- **One migration folder, embedded.** Migrations live in [services/internal/db/migrations](../../services/internal/db/migrations) as sequentially numbered goose SQL files with Up and Down sections. `db.Migrate` applies them from the binary (dev start-up, and the Kubernetes Job in LN-9.2). `make db-migrate` runs the goose CLI on the same files.
- **CHECKs, not enum types.** `unit`, `kind` and `sign` are `text`/`smallint` with CHECK constraints, and keys, user IDs, labels, titles and reasons have format or non-blank checks. A later migration can change a CHECK with a plain `ALTER`, which is harder for enum types. values-api still validates first and returns 422; the constraints are the backstop.
- **Append-only history, enforced for everyone.** `UPDATE`, `DELETE` and `TRUNCATE` on `value_changes` are revoked from `PUBLIC`, and triggers reject them (SQLSTATE 42501). The triggers matter because the local database user is a superuser and the table owner, which grants alone don't restrict.
- **History rows have no foreign key to `source_values`.** History must outlive a deleted value, and a key would stop LN-2.4 from deleting a value that was edited but no longer appears in any report.
- **Creating a value writes no history row.** A value starts at version 1, and history starts with the first edit, so `value_changes.version >= 2`. `UNIQUE (value_id, version)` makes "exactly one history row per edit" hold even under concurrent PATCHes (LN-2.5).
- **Reports reference values with `ON DELETE RESTRICT`.** `report_cells.value_id` and `report_cell_sum_inputs.value_id` block deleting a value in use (LN-2.4 turns that into a 409). Deleting a report cascades to its cells and sum inputs.
- **Cell rules.** A per-kind CHECK requires `text` for text cells, `value_id` for value_ref cells, and a `label` and neither of the others for sum cells. `UNIQUE (report_id, position)` is `DEFERRABLE INITIALLY IMMEDIATE`, so a reorder can swap positions in one transaction. The rules that every sum cell has at least one input and that `sum_cell_id` names a sum cell span two tables, so values-api checks them (LN-3.4) rather than a trigger.
- **Seed data is separate from migrations.** [seed.sql](../../services/internal/db/seed.sql) holds 3 users, 12 values and the 2 reports, with fixed IDs so tests, the fake API and bug reports can name them. Every insert has `ON CONFLICT … DO NOTHING`, so seeding again changes nothing and never overwrites edits. `make db-seed` applies it with psql, `db.Seed` applies it from Go, and `make db-reset` rolls back every migration, then migrates and seeds. Migrations stay schema-only, so a production-like database (LN-9.2) doesn't have to contain demo data.
- **sqlc output is committed and checked.** [sqlc.yaml](../../services/sqlc.yaml) reads the goose migrations as the schema and writes package `store` to [internal/store](../../services/internal/store). NUMERIC maps to `shopspring/decimal`, never float; uuid maps to `google/uuid`; timestamptz maps to `time.Time`. pgx scans decimals through their `sql.Scanner`, so no type registration is needed. `make lint-sqlc` ([check-sqlc.sh](../../scripts/ci/check-sqlc.sh)) generates into a temporary copy and diffs the whole folder, so CI fails on a stale, missing, extra or hand-edited file. `sqlc diff` alone misses extra files, such as output left behind after a query file is deleted.
- **Tests use real Postgres.** [internal/db/dbtest](../../services/internal/db/dbtest) starts one `postgres:16.15-alpine` container per test binary with testcontainers-go, builds two template databases (migrated, and migrated plus seeded), and gives each test its own clone (`CREATE DATABASE … TEMPLATE`) through `dbtest.New` or `dbtest.NewSeeded`. The clones are fast and isolated, so tests can run in parallel.

## Consequences

- `go test ./...` and `make test-go` need Docker, both locally and in CI. GitHub's Ubuntu runners have it, and the dev container reaches the host daemon.
- values-api must map constraint errors (23505, 23503, 23514) to 409 and 422 problem responses, and tests can assert SQLSTATEs directly.
- CI installs the pinned sqlc release in the Go job, with the version read from the Dockerfile like the other tools.
- Changing a unit, kind or key format needs a migration as well as a code change.
- Values deleted after being edited leave history rows whose `value_id` no longer resolves. History readers must not assume the join succeeds.
