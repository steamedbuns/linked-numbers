# Linked Numbers

A small financial-reporting app. A number is entered once, and every report that uses it updates live, with an audit trail.
Dart + OverReact on the frontend (`/web`), Go services on the backend (`/services`), and Postgres LISTEN/NOTIFY for real-time updates.
[linked_numbers_project_spec.md](linked_numbers_project_spec.md) is the source of truth for scope, stack, data model, API and story acceptance criteria. Read the relevant story before starting work on it.

## Running commands: always use the dev container

The host has only git, gh and Docker. **Go, Dart, Node, sqlc, goose, golangci-lint, psql, kind, kubectl, k6 and Playwright exist only in the dev container.**

- Run every toolchain command through `./dev run`, from the repo root or a subdirectory (the working directory is kept):
  - `./dev run go test -race ./...`
  - `./dev run bash -c 'cd web && dart analyze'`
- Never install toolchains, packages or global tools on the host. If a tool is missing, add it to [.devcontainer/Dockerfile](.devcontainer/Dockerfile) with a pinned `ARG` version, then run `./dev rebuild`.
- Run git and gh on the host as usual.
- Check the toolchain with `./dev run scripts/dev/doctor.sh` (add `--full` to also test Docker, Postgres and kind).
- The container uses host networking. Services started with compose, testcontainers or kind are reachable at `localhost` from both the host and the container.
- The container can use the host Docker daemon, so `docker` commands (including `docker rm` and `docker volume rm`) affect the real host. Only remove containers, images or volumes that this project created.

## Repo layout

Some of these folders don't exist yet; they are created in LN-1.1.

- `/web`: Dart 3 + OverReact app (function components + hooks; the history panel is a `UiComponent2` class component), OverReact Redux, built_value.
- `/services`: one Go module, with one `cmd/<name>` per binary: `values-api`, `realtime-gateway`, `explain-api`.
- `/api/openapi.yaml`: the API source of truth. Update it with every endpoint change.
- `/deploy`: Docker Compose, Kustomize (kind), and observability config.
- `/docs/adr`: ADRs. Add one for every new decision.
- `.devcontainer/`, `dev`, `scripts/dev/`: the dev environment (LN-0).

## Rules that are easy to break

- **Never use floating point for money.** Use `decimal` in Dart, `shopspring/decimal` in Go, and `NUMERIC(20,4)` in Postgres. Amounts travel as JSON strings (`"1250000.0000"`).
- Every value edit writes the new value, exactly one `value_changes` row and a `pg_notify('values_changed', …)`, all in one transaction. `value_changes` is append-only.
- Only values-api reads or writes the tables. The gateway only LISTENs.
- Use parameterized SQL only, via sqlc. After changing a query or migration, regenerate (`./dev run bash -c 'cd services && sqlc generate'`) and commit the output. CI fails if it's stale.
- Generated Dart code (`*.g.dart`) comes from `dart run build_runner build`. Don't edit it by hand.
- Never commit secrets. The LLM key lives in `.env`, which is git-ignored (template: `.env.example`).
- Every REST call sends `X-User-Id` and `traceparent`. Errors use RFC 9457 problem+json.

## Workflow

- Name branches `LN-<epic>_<story>/<short_description>` (e.g. `LN-1_1/initial_setup`), and put the story ID in the PR title.
- Keep PRs under about 400 changed lines. Each PR needs a description and a self-review, plus screenshots for UI changes.
- Definition of done (spec, "Definition of done"): every acceptance criterion has an automated test, lint and format are clean, new endpoints are in OpenAPI, and new code paths emit spans and structured logs.
- Before calling work done, run the relevant checks inside the container, for example:
  - Go: `./dev run bash -c 'cd services && golangci-lint run && go test -race ./...'`
  - Dart: `./dev run bash -c 'cd web && dart analyze && dart run build_runner test'`
