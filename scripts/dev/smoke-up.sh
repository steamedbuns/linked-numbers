#!/usr/bin/env bash
# Smoke test for the monorepo skeleton (LN-1.1). Run inside the dev container:
#   ./dev run scripts/dev/smoke-up.sh
# Checks the project folders, starts the stack with `make up`, checks that
# Postgres and Jaeger answer, lints the API spec, then runs `make down`.
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

curl -fsS -o /dev/null --retry 30 --retry-delay 1 --retry-all-errors http://localhost:16686 \
  || fail "jaeger UI did not answer at localhost:16686 within 30s"
pass "jaeger UI answers"

redocly lint --format=summary api/openapi.yaml >/dev/null 2>&1 || fail "api/openapi.yaml does not lint"
pass "api/openapi.yaml lints"

echo "smoke: all checks passed"
