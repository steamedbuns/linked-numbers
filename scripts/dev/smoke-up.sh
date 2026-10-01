#!/usr/bin/env bash
# Smoke test for the monorepo skeleton (LN-1.1). Run inside the dev container:
#   ./dev run scripts/dev/smoke-up.sh
# Checks the project folders, starts the stack with `make up`, checks that
# Postgres and Jaeger answer, migrates and seeds the database, lints the API
# spec, then runs `make down` (the data is kept).
set -euo pipefail
cd "$(dirname "$0")/../.."

fail() { echo "smoke: FAIL: $*" >&2; exit 1; }
pass() { echo "smoke: ok   $*"; }

for path in web services api deploy docs api/openapi.yaml deploy/compose/compose.yaml; do
  [[ -e "$path" ]] || fail "missing $path"
done
pass "project folders"

trap 'make down >/dev/null 2>&1 || true' EXIT
make up
pass "make up"

db_url="postgres://linked_numbers:linked_numbers@localhost:5432/linked_numbers?sslmode=disable"
[[ "$(psql "$db_url" -tAc 'select 1')" == 1 ]] || fail "postgres did not answer at localhost:5432"
pass "postgres answers"

make db-migrate db-seed >/dev/null 2>&1 || fail "make db-migrate db-seed"
# Count the seed's own rows by their fixed IDs (services/internal/db/seed.sql),
# so values and reports added later by hand don't matter.
seed_counts="$(psql "$db_url" -tAc "select
  (select count(*) from source_values where id::text like '00000000-0000-4000-a000-0000000001%'),
  (select count(*) from reports where id::text like '00000000-0000-4000-a000-0000000002%')")"
[[ "$seed_counts" == "12|2" ]] || fail "expected 12 seed values and 2 seed reports, got $seed_counts"
pass "migrations and seed data (12 values, 2 reports)"

curl -fsS -o /dev/null --retry 30 --retry-delay 1 --retry-all-errors http://localhost:16686 \
  || fail "jaeger UI did not answer at localhost:16686 within 30s"
pass "jaeger UI answers"

redocly lint --format=summary api/openapi.yaml >/dev/null 2>&1 || fail "api/openapi.yaml does not lint"
pass "api/openapi.yaml lints"

echo "smoke: all checks passed"
