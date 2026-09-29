#!/usr/bin/env bash
# Smoke test for the Dart app shell (LN-1.3). Run inside the dev container:
#   ./dev run scripts/dev/smoke-web.sh [--screenshot=path.png]
# Starts `webdev serve` on port 8080, loads the page in headless Chrome and
# checks that the OverReact App component rendered, then stops the server.
# Uses a fixed port, so don't run it while `make serve-web` is running.
set -euo pipefail
cd "$(dirname "$0")/../.."

fail() { echo "smoke-web: FAIL: $*" >&2; exit 1; }
pass() { echo "smoke-web: ok   $*"; }

screenshot=""
for arg in "$@"; do
  case "$arg" in
    --screenshot=*) screenshot="$(realpath -m "${arg#--screenshot=}")" ;;
    *) fail "unknown argument: $arg" ;;
  esac
done

url=http://localhost:8080/
! curl -fsS -o /dev/null "$url" 2>/dev/null || fail "something is already serving $url; stop it first"
log="$(mktemp)"

# webdev is a wrapper script around a Dart process, so run it in its own
# session and stop the whole process group on exit.
setsid bash -c 'cd web && exec webdev serve web:8080' >"$log" 2>&1 &
server=$!
trap 'kill -- "-$server" 2>/dev/null || true; wait "$server" 2>/dev/null || true; rm -f "$log"' EXIT

curl -fs -o /dev/null --retry 180 --retry-delay 1 --retry-all-errors "$url" \
  || { cat "$log" >&2; fail "webdev did not serve $url within 180s"; }
pass "webdev serves $url"

node scripts/dev/chrome-check.mjs "$url" '#app h1' 'Linked Numbers' ${screenshot:+"$screenshot"} \
  || fail "App component did not render at $url"
pass "App component renders"
[[ -z "$screenshot" ]] || pass "screenshot saved to $screenshot"

echo "smoke-web: all checks passed"
