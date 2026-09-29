# 0001. Monorepo layout and local stack

- Status: Accepted
- Date: 2026-09-29

## Context

Linked Numbers has a Dart web app, three Go services, an OpenAPI contract and deployment config. Changes often span several of them (a new endpoint touches the spec, a service and the Dart client). Developers also need Postgres and a trace backend locally from the first week.

## Decision

- One repository with top-level folders `web/`, `services/` (one Go module, one `cmd/<name>` per binary), `api/` (`openapi.yaml`), `deploy/` and `docs/`.
- Local infrastructure runs in Docker Compose (`deploy/compose/compose.yaml`), driven by a root `Makefile` (`make up`, `make down`, `make clean`).
- Jaeger v2 all-in-one is the local trace backend and accepts OTLP directly on 4317/4318. The OTel Collector, Prometheus and Grafana join in LN-7 without changing the services' OTLP endpoint setting.
- Images are pinned to exact tags.

## Consequences

- One PR can change the contract and both of its ends together, and CI sees the whole system.
- The compose stack uses fixed ports (5432, 16686, 4317, 4318), so only one checkout or worktree can run it at a time.
- Jaeger all-in-one keeps traces in memory; they are lost when the stack stops, which is fine for development.
