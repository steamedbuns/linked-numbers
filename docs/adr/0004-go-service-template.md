# 0004. Go service template

- Status: Accepted
- Date: 2026-09-29

## Context

LN-1.4 asks for one way to start every Go service. Config comes from env vars, logs are JSON via slog, `/healthz` and `/readyz` work, and on SIGTERM the service shuts down gracefully within 10 seconds. The spec also wants a trace_id on every log line, but the OpenTelemetry SDK only arrives in LN-7.1. LN-1.5 (database) and every LN-2 story build on this template.

## Decision

- **One package, [services/internal/service](../../services/internal/service).** A binary's `main` is one line: `service.Service{Name, DefaultAddr, Routes, Ready}.Main()`. `Main` loads config, builds the logger, handles SIGINT and SIGTERM, and exits 1 on failure. `Run` takes a context and a listener, so tests drive it without signals or fixed ports.
- **Config from env vars, stdlib only.** `LoadConfig` reads `HTTP_ADDR`, `LOG_LEVEL` and `SHUTDOWN_TIMEOUT` through an injected `getenv` and reports every invalid variable at once. New settings (such as `DATABASE_URL` in LN-1.5) are new fields. We don't use a config library.
- **JSON logs with trace IDs.** [internal/logging](../../services/internal/logging) wraps `slog.JSONHandler` in a handler that adds `trace_id` and `span_id` from the context's OpenTelemetry span context. sloglint enforces `no-global: all` and `context: scope`, so code logs through an injected logger and passes ctx.
- **OTel API now, SDK later.** [internal/httpx](../../services/internal/httpx) middleware extracts the W3C `traceparent` with the OTel propagator, or generates random IDs when there is none, then writes one access line per request. LN-7.1 adds the SDK and otelhttp, which create real spans in the same context; the log handler stays as it is.
- **Health semantics.** `/healthz` is liveness: 200 while the process serves HTTP, with no dependency checks, so a database outage doesn't restart pods. `/readyz` runs the service's named `Check`s under one 2-second timeout and returns 503 problem+json naming the failures. Probe requests log at debug.
- **Shutdown within 10 seconds.** On SIGTERM, `Run` calls `http.Server.Shutdown` with `SHUTDOWN_TIMEOUT` (default and maximum 10s). That stops accepting connections and waits for in-flight requests. If time runs out, it closes the remaining connections and exits 1. There is no in-app drain delay. In Kubernetes (LN-9.2), a `preStop` sleep covers the time endpoints take to stop routing to the pod.
- **Init for config-dependent setup.** (Added in LN-1.5.) `Service.Init(ctx, cfg, logger)` runs inside `Run` before serving and returns `Deps`: routes, readiness checks, and a `Close` that runs after shutdown. values-api uses it to open its pgx pool, run migrations when `MIGRATE_ON_START` is set, and register `postgres` and `schema` checks. An Init error closes the listener and exits 1. `DATABASE_URL` and `MIGRATE_ON_START` are shared `Config` fields, and each service decides whether it needs them.
- **Ports.** values-api listens on 8081, because `webdev serve` has 8080 and every checkout shares the host network. realtime-gateway and explain-api get 8082 and 8083.

## Consequences

- Services started with `Service.Main` all look the same from outside, with the same probes, log shape and shutdown behavior.
- Until LN-7.1, generated trace IDs appear only in logs. No spans are exported.
- `Logger.WithGroup` nests `trace_id` inside the group, so code uses `slog.Group` attributes instead of grouped loggers.
- ServeMux's built-in 404 and 405 replies are still plain text. They move to problem+json with the first real routes (LN-2.1).
