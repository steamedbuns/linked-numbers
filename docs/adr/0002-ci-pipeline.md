# 0002. CI pipeline

- Status: Accepted
- Date: 2026-09-29

## Context

Broken code must not reach `main` (LN-1.2). CI has to run Go and Dart lint and unit tests on every PR in under 8 minutes, and use the same tool versions developers get from the dev container ([.devcontainer/Dockerfile](../../.devcontainer/Dockerfile)). More checks will come later: integration, e2e and image builds.

## Decision

- GitHub Actions ([.github/workflows/ci.yml](../../.github/workflows/ci.yml)) runs on every PR to `main` and on every push to `main`.
- Jobs install their tools with the official setup actions (`setup-go`, `setup-dart`, `golangci-lint-action`) instead of running in the dev container image. That image carries Chrome, Node, kind and more, and building it cold would use up much of the 8-minute budget. To keep a single source of truth, [scripts/ci/tool-versions.sh](../../scripts/ci/tool-versions.sh) reads the versions from the Dockerfile `ARG`s.
- Every job runs a `make` target (`lint-go`, `lint-sqlc`, `test-go`, `lint-web`, `test-web`, `lint-api`), the same one developers run with `./dev run make check`. When a check changes, the Makefile changes and the workflow does not.
- One aggregator job, `CI`, needs every other job and fails unless they all succeeded. It is the only required status check in the `main` ruleset ([.github/rulesets/main.json](../../.github/rulesets/main.json)), so jobs can be added or renamed without touching the ruleset.
- Every job has `timeout-minutes: 8`. The jobs run in parallel, so a run that goes over the budget fails instead of passing slowly.
- Third-party actions are pinned to full commit SHAs, with the version in a comment.

## Consequences

- CI and local runs use the same versions, but not the same image. OS packages such as Chrome can differ between them. Browser tests (LN-1.3) will use the runner's Chrome unless we pin it.
- Changing the ruleset is a manual step: edit `main.json`, then apply it with `gh api --method PUT repos/steamedbuns/linked-numbers/rulesets/24195765 --input .github/rulesets/main.json`.
- SHA-pinned actions don't update themselves, so we bump them by hand.
