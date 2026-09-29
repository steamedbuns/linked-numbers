# linked-numbers
Linked numbers demo project

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

**VS Code:** open the repo in a WSL window and run **Dev Containers: Reopen in Container**. You need the Dev Containers extension.

**Terminal only:**

```bash
./dev
```

`./dev run <cmd>` runs a single command, `./dev rebuild` picks up Dockerfile changes, `./dev down` stops the container, and `./dev ps` lists containers.
Each checkout, including every git worktree, gets its own container. The image rebuilds automatically when `.devcontainer/` changes.

### 3. Check the toolchain

```bash
./dev run scripts/dev/doctor.sh --full
```

### How it works

- **Docker-outside-of-Docker.** The container uses the host's Docker socket. Compose stacks, testcontainers and kind clusters run as sibling containers on the host.
- **Host networking.** Every port you start (Postgres, Jaeger, `webdev serve`, and so on) is on `localhost`, and you can open it straight from the Windows browser.
- **Same-path mount.** The repo is mounted at the same absolute path it has on the host, so relative bind mounts in compose files work.
- **Cache volumes.** Go, pub, npm and Playwright caches live in the named volumes `ln-*` and survive rebuilds.
- **Secrets.** `.env` is created from [.env.example](.env.example) on first start. It is git-ignored; put the LLM key there.

### Uninstall

`./dev nuke` removes every dev container (for all worktrees), the images and the cache volumes. Optionally, also remove Docker with `sudo apt-get purge docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin`.

`make up` and the project folders are added in LN-1.1.
