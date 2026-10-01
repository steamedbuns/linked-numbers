#!/usr/bin/env bash
# Fails if services/internal/store, the committed sqlc output, differs from
# what `sqlc generate` writes now: a stale, missing, extra or hand-edited
# file. (`sqlc diff` misses extra files, e.g. after a query file is deleted.)
# It generates into a temporary copy, so the working tree is left alone.
set -euo pipefail
cd "$(dirname "$0")/../../services"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/internal"
cp sqlc.yaml "$tmp/"
cp -r internal/db "$tmp/internal/db"
(cd "$tmp" && sqlc generate)

if ! diff -ru --label committed --label generated internal/store "$tmp/internal/store"; then
  echo "sqlc: services/internal/store is stale. Regenerate and commit it:" >&2
  echo "  ./dev run bash -c 'cd services && sqlc generate'" >&2
  exit 1
fi
echo "sqlc: services/internal/store is up to date"
