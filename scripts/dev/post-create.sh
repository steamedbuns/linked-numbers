#!/usr/bin/env bash
# Runs once inside a freshly created dev container (VS Code postCreateCommand
# or ./dev). Idempotent, and safe before the LN-1 project folders exist.
set -euo pipefail
cd "$(dirname "$0")/../.."

# New named volumes can come up root-owned; hand them to vscode.
sudo chown vscode:vscode \
  "$HOME/go/pkg/mod" "$HOME/.cache/go-build" "$HOME/.pub-cache" \
  "$HOME/.npm" "$HOME/.cache/ms-playwright"

# The pub-cache volume outlives image rebuilds; keep webdev at the image's pin.
if ! dart pub global list 2>/dev/null | grep -qx "webdev ${WEBDEV_VERSION}"; then
  dart pub global activate webdev "${WEBDEV_VERSION}"
fi

[[ -f .env ]] || { [[ -f .env.example ]] && cp .env.example .env && echo "Created .env from .env.example"; }

if [[ -f services/go.mod ]]; then (cd services && go mod download); fi
if [[ -f web/pubspec.yaml ]]; then (cd web && dart pub get); fi
if [[ -f e2e/package.json ]]; then (cd e2e && npm ci && npx playwright install chromium); fi

echo "post-create done. Run scripts/dev/doctor.sh to check the toolchain."
