# linked-numbers
Linked numbers demo project

## Quick start

On a clean machine you need only Docker (see step 1 below for WSL2). From the repo root:

```bash
scripts/host/install-docker-wsl.sh   # once per machine, then open a new terminal
./dev run make up                    # builds the dev container on first run, then starts Postgres and Jaeger
./dev run scripts/dev/smoke-up.sh    # optional: checks the stack end to end
```

| Service | Where |
| --- | --- |
| Postgres 16 | `localhost:5432`, user/password/db `linked_numbers` (`./dev run make psql`) |
| Jaeger UI | http://localhost:16686 |
| OTLP (traces) | `localhost:4317` (gRPC), `localhost:4318` (HTTP) |

`./dev run make db-migrate` creates the schema in the local Postgres (see [services/README.md](services/README.md#database)).
`./dev run make down` stops the stack and keeps the data; `./dev run make clean` also deletes it. `./dev run make` lists every target.
The stack uses fixed ports, so run it from only one checkout or worktree at a time.

## Repo layout

| Folder | Contents |
| --- | --- |
| [`web/`](web/) | Dart 3 + OverReact app |
| [`services/`](services/) | Go module: `values-api`, `realtime-gateway`, `explain-api` |
| [`api/`](api/) | `openapi.yaml`, the API source of truth |
| [`deploy/`](deploy/) | Docker Compose now; Kustomize and observability config later |
| [`docs/adr/`](docs/adr/) | Architecture decision records |

## CI

[GitHub Actions](.github/workflows/ci.yml) runs Go lint and tests, Dart format, analyze and tests, and the OpenAPI lint on every PR, with tool versions read from the dev container's Dockerfile.
The `CI` check must pass before a PR can merge into `main`. To run the same checks locally:

```bash
./dev run make check
```

## Development environment

Everything except Docker runs inside one dev container image
([.devcontainer/Dockerfile](.devcontainer/Dockerfile)): Go, Dart + webdev, Chrome, Node + Playwright deps,
sqlc, goose, golangci-lint, k6, kind, kubectl, kustomize, psql, Redocly CLI, and the docker/compose CLI.
The host needs only Docker Engine. Tool versions are pinned as `ARG`s at the top of the Dockerfile.

### 1. One-time host setup (WSL2 Ubuntu)

```bash
scripts/host/install-docker-wsl.sh
```

Then open a new WSL terminal (or run `wsl --shutdown` from Windows) so the `docker` group applies.

### 2. Enter the dev environment

**VS Code:** open the repo in a WSL window (**WSL: Connect to WSL**, or `code .` from a WSL terminal), then run **Dev Containers: Reopen in Container**. You need the WSL and Dev Containers extensions.
Docker is installed only inside WSL; there's no Docker Desktop. If you start from a plain Windows window, you get `Executable 'docker' not found on PATH 'C:\WINDOWS\...'`. Either reopen through WSL, or add `"dev.containers.executeInWSL": true` to your VS Code user settings.

**Terminal only:**

```bash
./dev
```

`./dev run <cmd>` runs a single command, `./dev rebuild` picks up `.devcontainer/` changes, `./dev down` removes the container, and `./dev ps` lists containers.

`./dev` creates containers with the [devcontainer CLI](https://github.com/devcontainers/cli) from the same `devcontainer.json` VS Code uses. So VS Code and `./dev` share **one container per checkout**, named `linked-numbers-dev-<folder name>`, and whichever starts first creates it. Each git worktree gets its own container. Worktree folders need distinct names, because the name comes from the folder.

### 3. Check the toolchain

```bash
./dev run scripts/dev/doctor.sh --full
```

### How it works

- **Docker-outside-of-Docker.** The container uses the host's Docker socket. Compose stacks, testcontainers and kind clusters run as sibling containers on the host.
- **Host networking.** Every port you start (Postgres, Jaeger, `webdev serve`, and so on) is on `localhost`, and you can open it straight from the Windows browser.
- **Same-path mount.** The repo is mounted at the same absolute path it has on the host, so relative bind mounts in compose files work. For a worktree started with `./dev`, the main repo's `.git` is mounted too, so git works inside it. A worktree container created by VS Code doesn't get that mount; run `./dev rebuild` in the worktree if you need git inside it.
- **Cache volumes.** Go, pub, npm and Playwright caches live in the named volumes `ln-*` and survive rebuilds.
- **Secrets.** `.env` is created from [.env.example](.env.example) on first start. It is git-ignored; put the LLM key there.

### Uninstall

`./dev nuke` removes every dev container (for all worktrees), the images and the cache volumes. Optionally, also remove Docker with `sudo apt-get purge docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin`.
