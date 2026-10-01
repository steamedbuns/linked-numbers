#!/usr/bin/env bash
# Prints the pinned tool versions from .devcontainer/Dockerfile as name=value
# lines (e.g. go=1.27.1, golangci_lint=2.14.0), so CI uses the same versions as the dev container.
# In GitHub Actions they also go to $GITHUB_OUTPUT as step outputs.
set -euo pipefail
cd "$(dirname "$0")/../.."

out="$(sed -nE 's/^ARG (GO|DART|GOLANGCI_LINT|SQLC|REDOCLY)_VERSION=(.+)$/\1=\2/p' .devcontainer/Dockerfile \
  | tr '[:upper:]' '[:lower:]')"
[[ "$(wc -l <<<"$out")" == 5 ]] || { echo "tool-versions: expected 5 versions, got:" >&2; echo "$out" >&2; exit 1; }

echo "$out"
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then echo "$out" >>"$GITHUB_OUTPUT"; fi
