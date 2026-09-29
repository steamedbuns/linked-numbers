#!/usr/bin/env bash
# Checks the dev container toolchain. Run inside the container:
#   ./dev run scripts/dev/doctor.sh [--full]
# --full also starts a kind cluster and a throwaway Postgres (~1-2 min).
set -uo pipefail

fail=0
ok()  { printf '  \e[32mok\e[0m   %-16s %s\n' "$1" "$2"; }
bad() { printf '  \e[31mFAIL\e[0m %-16s %s\n' "$1" "$2"; fail=1; }

check() { # name, command...
  local name="$1"; shift
  local out
  if out="$("$@" 2>&1 | head -n1)"; then ok "$name" "$out"; else bad "$name" "$out"; fi
}

echo "Tools"
check go            go version
check gopls         gopls version
check dlv           dlv version
check golangci-lint golangci-lint version
check govulncheck   govulncheck -version
check gotestsum     gotestsum --version
check sqlc          sqlc version
check goose         goose --version
check dart          dart --version
check webdev        webdev --version
check chrome        google-chrome --version
check node          node --version
check npm           npm --version
check redocly       redocly --version
check k6            k6 version
check kind          kind version
check kubectl       kubectl version --client
check kustomize     kustomize version
check psql          psql --version
check docker        docker --version
check compose       docker compose version
check buildx        docker buildx version
check make          make --version
check jq            jq --version
check yq            yq --version
check shellcheck    shellcheck --version
check gh            gh --version

echo "Runtime"
check docker-access docker info --format 'server {{.ServerVersion}}'
check hello-world   bash -c 'docker run --rm hello-world >/dev/null && echo "container ran"'
check chrome-run    bash -c 'out=$(google-chrome --headless=new --disable-gpu --dump-dom about:blank 2>/dev/null) && [[ $out == *"<html"* ]] && echo "headless render ok"'

if [[ "${1:-}" == "--full" ]]; then
  echo "Full checks"
  check postgres bash -c '
    docker pull -q postgres:16 >/dev/null || exit 1
    cid=$(docker run -d --rm -e POSTGRES_PASSWORD=pw -p 127.0.0.1::5432 postgres:16) || exit 1
    trap "docker rm -f $cid >/dev/null" EXIT
    port=$(docker port "$cid" 5432 | head -n1 | cut -d: -f2)
    for _ in $(seq 30); do
      if v=$(PGPASSWORD=pw psql -h 127.0.0.1 -p "$port" -U postgres -tAc "show server_version" 2>/dev/null); then
        echo "postgres $v reachable on localhost:$port"; exit 0
      fi
      sleep 1
    done
    echo "postgres not reachable on localhost:$port after 30s"; exit 1'
  check kind-cluster bash -c '
    kind create cluster --name ln-doctor --wait 120s >/dev/null 2>&1 || { kind delete cluster --name ln-doctor >/dev/null 2>&1; exit 1; }
    trap "kind delete cluster --name ln-doctor >/dev/null 2>&1" EXIT
    kubectl get nodes --no-headers | head -n1'
fi

echo
if [[ $fail == 0 ]]; then echo "All checks passed."; else echo "Some checks failed."; fi
exit $fail
