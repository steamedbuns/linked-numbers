# Linked Numbers dev image: the single source of truth for the project toolchain.
# Used by .devcontainer/devcontainer.json (VS Code) and ./dev (terminal).
#
# Ubuntu 24.04 is used (not the host's 26.04) because Playwright, Chrome for
# Testing and every upstream apt repo below have first-class support for it.
FROM mcr.microsoft.com/devcontainers/base:3.0.8-ubuntu24.04

# ---- Pinned versions (bump here, then `./dev rebuild`) ----------------------
ARG GO_VERSION=1.27.1
# Newest Dart that build_web_compilers accepts (it caps the SDK at <3.14).
ARG DART_VERSION=3.13.5
ARG NODE_VERSION=24.21.0
ARG CHROME_VERSION=154.0.8037.57
ARG PLAYWRIGHT_VERSION=1.63.0
ARG GOLANGCI_LINT_VERSION=2.14.0
ARG SQLC_VERSION=1.31.1
ARG GOOSE_VERSION=3.28.0
ARG GOPLS_VERSION=0.23.0
ARG DELVE_VERSION=1.27.2
ARG GOVULNCHECK_VERSION=1.8.0
ARG GOTESTSUM_VERSION=1.13.0
ARG WEBDEV_VERSION=4.0.3
ARG REDOCLY_VERSION=2.55.0
ARG DEVCONTAINER_CLI_VERSION=0.89.0
ARG K6_VERSION=2.3.0
ARG KIND_VERSION=0.33.0
ARG KUBECTL_VERSION=1.37.1
ARG KUSTOMIZE_VERSION=5.8.1
ARG YQ_VERSION=4.54.1
ARG POSTGRES_CLIENT_MAJOR=16

SHELL ["/bin/bash", "-o", "pipefail", "-c"]
ENV DEBIAN_FRONTEND=noninteractive

# ---- apt: base tools, docker CLI, psql ---------------------------------------
RUN install -m 0755 -d /etc/apt/keyrings \
 && curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc \
 && echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu noble stable" \
      > /etc/apt/sources.list.d/docker.list \
 && curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc -o /etc/apt/keyrings/pgdg.asc \
 && echo "deb [signed-by=/etc/apt/keyrings/pgdg.asc] https://apt.postgresql.org/pub/repos/apt noble-pgdg main" \
      > /etc/apt/sources.list.d/pgdg.list \
 && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg -o /etc/apt/keyrings/githubcli.gpg \
 && echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/githubcli.gpg] https://cli.github.com/packages stable main" \
      > /etc/apt/sources.list.d/github-cli.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends \
      build-essential make jq shellcheck unzip xz-utils bash-completion \
      docker-ce-cli docker-buildx-plugin docker-compose-plugin \
      postgresql-client-${POSTGRES_CLIENT_MAJOR} gh \
 && rm -rf /var/lib/apt/lists/*

# ---- Go ----------------------------------------------------------------------
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz

# ---- Dart --------------------------------------------------------------------
RUN curl -fsSLo /tmp/dart.zip \
      "https://storage.googleapis.com/dart-archive/channels/stable/release/${DART_VERSION}/sdk/dartsdk-linux-x64-release.zip" \
 && unzip -q /tmp/dart.zip -d /usr/lib \
 && mv /usr/lib/dart-sdk /usr/lib/dart \
 && chmod -R a+rX /usr/lib/dart \
 && rm /tmp/dart.zip

# ---- Node.js -----------------------------------------------------------------
RUN curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz" \
      | tar -C /usr/local --strip-components=1 -xJ \
 && npm install -g "@redocly/cli@${REDOCLY_VERSION}" "@devcontainers/cli@${DEVCONTAINER_CLI_VERSION}" \
 && npx -y "playwright@${PLAYWRIGHT_VERSION}" install-deps chromium \
 && rm -rf /var/lib/apt/lists/* /root/.npm

# ---- Chrome for Testing (pinned; used by `dart run build_runner test -p chrome`)
# The wrapper adds --no-sandbox: Chrome's sandbox needs user namespaces, which
# Docker's default seccomp profile blocks for non-root users.
RUN curl -fsSLo /tmp/chrome.zip \
      "https://storage.googleapis.com/chrome-for-testing-public/${CHROME_VERSION}/linux64/chrome-linux64.zip" \
 && unzip -q /tmp/chrome.zip -d /opt \
 && mv /opt/chrome-linux64 /opt/chrome \
 && rm /tmp/chrome.zip \
 && printf '#!/bin/sh\nexec /opt/chrome/chrome --no-sandbox "$@"\n' > /usr/local/bin/google-chrome \
 && chmod +x /usr/local/bin/google-chrome

# ---- Single-binary tools -----------------------------------------------------
RUN curl -fsSL "https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VERSION}/golangci-lint-${GOLANGCI_LINT_VERSION}-linux-amd64.tar.gz" \
      | tar -xz -C /usr/local/bin --strip-components=1 "golangci-lint-${GOLANGCI_LINT_VERSION}-linux-amd64/golangci-lint" \
 && curl -fsSL "https://github.com/sqlc-dev/sqlc/releases/download/v${SQLC_VERSION}/sqlc_${SQLC_VERSION}_linux_amd64.tar.gz" \
      | tar -xz -C /usr/local/bin sqlc \
 && curl -fsSL "https://github.com/grafana/k6/releases/download/v${K6_VERSION}/k6-v${K6_VERSION}-linux-amd64.tar.gz" \
      | tar -xz -C /usr/local/bin --strip-components=1 "k6-v${K6_VERSION}-linux-amd64/k6" \
 && curl -fsSLo /usr/local/bin/kind "https://kind.sigs.k8s.io/dl/v${KIND_VERSION}/kind-linux-amd64" \
 && curl -fsSLo /usr/local/bin/kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
 && curl -fsSL "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" \
      | tar -xz -C /usr/local/bin kustomize \
 && curl -fsSLo /usr/local/bin/yq "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64" \
 && chmod +x /usr/local/bin/kind /usr/local/bin/kubectl /usr/local/bin/yq

# ---- Environment -------------------------------------------------------------
ENV GOTOOLCHAIN=local \
    WEBDEV_VERSION=${WEBDEV_VERSION} \
    GOPATH=/home/vscode/go \
    PUB_CACHE=/home/vscode/.pub-cache \
    CHROME_EXECUTABLE=/usr/local/bin/google-chrome \
    PLAYWRIGHT_BROWSERS_PATH=/home/vscode/.cache/ms-playwright \
    PATH=/home/vscode/go/bin:/home/vscode/.pub-cache/bin:/usr/local/go/bin:/usr/lib/dart/bin:${PATH}

# ---- Per-user tools (installed as vscode so ownership is right) --------------
USER vscode
RUN go install "golang.org/x/tools/gopls@v${GOPLS_VERSION}" \
 && go install "github.com/go-delve/delve/cmd/dlv@v${DELVE_VERSION}" \
 && go install "golang.org/x/vuln/cmd/govulncheck@v${GOVULNCHECK_VERSION}" \
 && go install "gotest.tools/gotestsum@v${GOTESTSUM_VERSION}" \
 && go install "github.com/pressly/goose/v3/cmd/goose@v${GOOSE_VERSION}" \
 && go clean -cache -modcache \
 && dart --disable-analytics \
 && dart pub global activate webdev "${WEBDEV_VERSION}" \
 && mkdir -p /home/vscode/.cache/go-build /home/vscode/go/pkg/mod /home/vscode/.npm "${PLAYWRIGHT_BROWSERS_PATH}"

USER root
COPY docker-init.sh /usr/local/bin/docker-init.sh
RUN chmod +x /usr/local/bin/docker-init.sh
ENTRYPOINT ["/usr/local/bin/docker-init.sh"]
CMD ["sleep", "infinity"]
