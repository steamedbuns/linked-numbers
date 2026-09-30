# Linked Numbers Project Spec

Sep 29, 2026 · @Qian

Linked Numbers is a small financial-reporting app. A number is entered once, and every report that uses it updates live, with a full audit trail. It uses Dart + OverReact on the frontend and Go services on the backend, with OpenTelemetry tracing and one AI feature that cites its sources.

## Product brief

**Problem.** Finance teams copy the same figure (e.g. Q3 revenue) into many documents. When it changes late, someone has to find and fix every copy, and auditors ask who changed it and why.

**Users**

- **Preparer**: enters and edits source values.
- **Report author**: builds reports whose cells link to those values.
- **Reviewer/auditor**: reads the change history and asks "why did this move?"

**In scope**

- A table of source values (label, amount, unit, period).
- Reports made of ordered cells: text, a linked value, or a sum of chosen values, each added or subtracted.
- Live updates: an edit shows up in every open report within 1 second.
- An append-only history of every change, with author, time, old → new and a required reason.
- "Explain this change": an AI summary that cites only the history rows it used.
- Tracing from the browser through every service, plus a KPI dashboard.

**Non-goals**

- Real authentication or SSO: a user picker is enough, and every call still sends a user ID.
- Rich text, spreadsheets with arbitrary formulas, multi-tenant setup, XBRL export.
- Production cloud hosting: local Kubernetes is the target, with an optional AWS stretch.

**Done means**

- [ ] The 5-minute demo works: build a report with a margin line in the builder → edit a value → two reports update live → open the history → get an explanation with citations → show the trace in Jaeger.
- [ ] CI is green, with tests at every layer.
- [ ] The README has the architecture diagram, 3+ ADRs and a one-paragraph summary for non-engineers.

## Architecture

&#91;embedded content: Linked Numbers architecture · 4-step edit path + Explain\]

1. Tab A sends `PATCH /values/{id}` with the new amount, a reason and the expected version.
2. values-api writes the new value and a history row, and calls `pg_notify`, all in one transaction.
3. When the transaction commits, Postgres delivers the notification to realtime-gateway, which is listening on `values_changed`.
4. The gateway pushes the event to every WebSocket client that has an affected report open. Tab B updates the cell and highlights it.

Explain (dashed): the history panel calls explain-api. explain-api fetches the rows through values-api, sends them to the LLM, and rejects any answer that cites rows it wasn't given. Only values-api reads or writes the tables. The gateway only listens for notifications. Postgres doesn't store notifications, so a client that misses one catches up by refetching when it reconnects.

## Tech stack requirements

Every choice below is required. Use the latest stable release of each unless a version is given. Pin exact versions in `pubspec.lock` and `go.sum`.

**Frontend (`/web`)**

| Need | Required choice | Notes |
| --- | --- | --- |
| Language | Dart 3.x, sound null safety | Records and sealed classes for cell kinds and events |
| UI components | [over\_react](https://github.com/Workiva/over_react), function components + hooks | The history panel is a class component (`UiComponent2`) |
| State | OverReact Redux | Slices: `values`, `reports`, `history`, `connection` |
| Immutable models | `built_value` + `built_collection` | Generated with build\_runner |
| Money math | `decimal` package | Never `double` for amounts |
| HTTP / WebSocket | `package:http`, `package:web_socket_channel` | Typed client wrapper; reconnect with backoff |
| Build / dev | build\_runner, `webdev serve`, [dart\_dev](https://github.com/Workiva/dart_dev) | `analysis_options.yaml` with strict lints |
| Tests | `package:test`, [react\_testing\_library](https://github.com/Workiva/react_testing_library) Dart bindings, `mocktail` | Run with `dart run build_runner test` |
| Tracing | [opentelemetry-dart](https://github.com/Workiva/opentelemetry-dart) | Sends a `traceparent` header on every request |

**Backend (`/services`)**

| Need | Required choice | Notes |
| --- | --- | --- |
| Language | Go 1.22+ | Uses the 1.22 `net/http` routing patterns, so no router library is needed |
| Services | `values-api`, `realtime-gateway`, `explain-api` | One Go module, one `cmd/` folder per binary |
| Database | PostgreSQL 16 | `NUMERIC(20,4)` for amounts |
| DB access | `pgx` v5 + `sqlc` | SQL-first, type-safe queries |
| Migrations | `goose` | Run on startup in dev, as a Job in Kubernetes |
| Decimals | `shopspring/decimal` | Serialized as strings in JSON |
| Messaging | Postgres LISTEN/NOTIFY via pgx | Channel `values_changed`; best effort, so clients refetch on reconnect. Payload must stay under Postgres's 8,000-byte limit |
| WebSocket | `coder/websocket` | One connection per browser tab; subscribes by report ID |
| Validation / API docs | OpenAPI 3.1 spec checked into `/api` | The Dart client follows the spec |
| Logging | `log/slog`, JSON output | trace\_id on every line |
| Tests | `testing`, table tests, `testcontainers-go` for Postgres | `go test -race ./...` in CI |

**Platform and tooling**

| Need | Required choice | Notes |
| --- | --- | --- |
| Local run | Docker Compose | One `make up` starts everything |
| Kubernetes | kind or k3d + Kustomize | Stretch: AWS EKS or ECS with RDS |
| Observability | OpenTelemetry Collector → Jaeger (traces), Prometheus (metrics), Grafana (dashboards) | Dashboards stored as JSON in the repo |
| E2E / load | Playwright; k6 | k6 checks the latency targets |
| CI | GitHub Actions | Jobs: lint, unit, integration, e2e, build images |
| AI | Any LLM API with JSON/structured output | Key read from env; a fake client for tests |
| Docs | README, ADRs in `/docs/adr`, Mermaid diagrams |  |

## Data model

Six Postgres tables. A value's current amount lives in `source_values`. In the same transaction, every change to it adds one row to `value_changes` and sends a notification on channel `values_changed`.

| Table | Key columns | Rules |
| --- | --- | --- |
| `users` | `id` text (e.g. `u_ana`), `display_name`, `created_at` | The user picker's list. `updated_by`, `changed_by` and `created_by` reference it, so `X-User-Id` must name a known user |
| `source_values` | `id` UUID, `key` text unique (e.g. `rev.q3.2026`), `label`, `amount` NUMERIC(20,4), `unit` (USD, %, count), `period`, `version` int, `updated_at`, `updated_by` | `version` goes up by 1 on each edit, which is how conflicting edits are detected |
| `value_changes` | `id` bigserial, `value_id`, `old_amount`, `new_amount`, `version`, `reason` text, `changed_by`, `changed_at` | Append-only: no UPDATE or DELETE grants. `reason` must be non-empty. `id` is the event ID in notifications |
| `reports` | `id` UUID, `title`, `created_by`, `created_at` |  |
| `report_cells` | `id`, `report_id`, `position` int, `kind` (`text` \| `value_ref` \| `sum`), `text`, `value_id` (for `value_ref`), `label` | Unique (`report_id`, `position`). A `sum` cell's total comes only from its rows in `report_cell_sum_inputs`, never from its position |
| `report_cell_sum_inputs` | `sum_cell_id`, `value_id`, `sign` smallint | Required: at least one row per sum cell. `sign` is +1 or −1 (story LN-3.4) |

**Rules for all tables**

- Amounts go over the wire as strings (`"1250000.0000"`) and are parsed with decimal libraries on both ends.
- Deleting a value that any report uses is blocked (409), and the error names the reports that use it.
- Seed data: 12 values (revenue, COGS and opex for Q1–Q4) and 2 reports ("Q3 earnings release", "Board deck: financial summary") that share 4 values.

## API contract

All REST calls are JSON and send `X-User-Id` and `traceparent` headers. Errors follow RFC 9457 problem+json. The OpenAPI spec in `/api/openapi.yaml` is the source of truth; this table is a summary of it.

| Method + path | Service | Purpose | Key responses |
| --- | --- | --- | --- |
| `GET /values` | values-api | List values; filter by `period` | 200 |
| `POST /values` | values-api | Create a value | 201; 409 if the key exists |
| `PATCH /values/{id}` | values-api | Edit the amount; body has `amount`, `reason`, `expectedVersion` | 200; 409 on a version conflict, returning the current value; 422 if the reason is blank |
| `DELETE /values/{id}` | values-api | Delete a value if unused | 204; 409 listing the reports that use it |
| `GET /values/{id}/history` | values-api | Change history, newest first, paginated with a cursor | 200 |
| `GET /reports` / `POST /reports` | values-api | List or create reports | 200 / 201 |
| `GET /reports/{id}` | values-api | Report with cells resolved to current amounts and sums computed | 200 |
| `PUT /reports/{id}/cells` | values-api | Replace the ordered cell list | 200; 422 if a cell is invalid |
| `GET /values/{id}/usages` | values-api | Reports and cells that link to a value | 200 |
| `POST /values/{id}/explain` | explain-api | AI summary of a change range; body has `fromVersion`, `toVersion` | 200 with `summary` + `citations[]`; 502 if the model's answer fails validation |
| `GET /ws?reports=a,b` | realtime-gateway | WebSocket that streams events for those reports | 101 |
| `GET /healthz`, `/readyz`, `/metrics` | all | Health and Prometheus metrics | 200 |

**Real-time events.** values-api sends each event with `pg_notify('values_changed', …)` inside the edit transaction, so Postgres delivers it only if the edit commits. realtime-gateway listens on that channel and relays events to WebSocket clients:

```json
{"type": "value.changed", "eventId": 8812, "valueId": "…", "version": 7,
 "oldAmount": "1200000.0000", "newAmount": "1250000.0000",
 "changedBy": "u_ana", "changedAt": "2026-10-14T15:02:11Z",
 "affectedReports": ["…", "…"], "traceparent": "00-…-01"}
```

- Clients ignore any event whose `version` is not newer than what they hold, so duplicates and out-of-order events are harmless.
- On reconnect, the client refetches its open reports rather than replaying missed events. Postgres doesn't keep notifications, so this refetch is how missed events are recovered.

## Epics overview

There are 9 epics and 45 stories, totaling 139 points plus 5 stretch points. The frontend starts in week 1 against a fake API, so UI and service work can overlap.

| Epic | Goal | Target weeks | Stories | Points |
| --- | --- | --- | --- | --- |
| LN-1 Foundation | Repo, CI, app shells, database migrations | 1 (web), 4 (Go) | 5 | 13 |
| LN-2 Values service | Create, edit and delete values with a safe, audited history | 4–5 | 5 | 15 |
| LN-3 Reports and links | Reports whose cells link to values, and sums with explicit inputs | 5 | 4 | 13 |
| LN-4 Frontend editor | Values grid, report view and builder, history panel in OverReact | 1–3 | 7 | 26 |
| LN-5 Real-time propagation | An edit reaches every open report in under 1 second | 6 | 5 | 17 |
| LN-6 Testing and quality | Test plan, component, e2e and fault tests, coverage gates | 1–9 | 5 | 15 |
| LN-7 Observability and KPIs | One trace from click to other tab; KPI dashboard | 6–7 | 4 | 13 |
| LN-8 AI explain | Change summaries that cite only real history rows | 8 | 4 | 14 |
| LN-9 Deploy and docs | Kubernetes deploy, ADRs, README, demo | 9 | 6 | 13 (+5 stretch) |

Points are relative size (1, 2, 3, 5), not hours. The 29 stories marked **MVP** (98 points) make up the first release and the 5-minute demo. The rest follow it.

## Story tickets

Each ticket can be pasted into Jira or GitHub Issues as is. Every story also has to meet the definition of done (next section).

### LN-1 Foundation

| ID | Story | Acceptance criteria | Pts | MVP | Depends on |
| --- | --- | --- | --- | --- | --- |
| LN-1.1 | As a developer, I want a monorepo skeleton so that every part has a home. | Folders `/web`, `/services`, `/api`, `/deploy`, `/docs`. `make up` starts Postgres and Jaeger in Docker Compose. The README quick start works on a clean machine. | 2 | Yes | — |
| LN-1.2 | As a developer, I want CI on every PR so that broken code can't merge. | GitHub Actions runs Dart and Go lint and unit tests. Checks are required on `main`. A run finishes in under 8 minutes. | 3 | Yes | LN-1.1 |
| LN-1.3 | As a developer, I want a Dart app shell so that UI work can start. | `webdev serve` renders an OverReact `App` component. Strict lints pass. One component test passes under `build_runner test`. | 2 | Yes | LN-1.1 |
| LN-1.4 | As a developer, I want a Go service template so that all services start the same way. | Config comes from env vars. Logs are JSON via slog. `/healthz` and `/readyz` work. On SIGTERM the service shuts down gracefully within 10 seconds. | 3 | Yes | LN-1.1 |
| LN-1.5 | As a developer, I want migrations and seed data so that everyone starts from the same state. | goose migrations create all six tables. The seed has 12 values and 2 reports. CI fails if the checked-in `sqlc` output is stale. | 3 | Yes | LN-1.4 |

### LN-2 Values service

| ID | Story | Acceptance criteria | Pts | MVP | Depends on |
| --- | --- | --- | --- | --- | --- |
| LN-2.1 | As a preparer, I want to list and create values so that I can enter source numbers. | `GET /values` filters by period. `POST` checks key format and unit. Amounts go in and out as strings. A duplicate key returns 409. | 3 | Yes | LN-1.5 |
| LN-2.2 | As a preparer, I want to edit a value with a reason so that every change is explained. | `PATCH` needs `expectedVersion`; a stale version returns 409 with the current value. A blank reason returns 422. The value and history row are written, and the notification sent, in one transaction. | 5 | Yes | LN-2.1 |
| LN-2.3 | As a reviewer, I want a value's history so that I can see who changed what. | Newest first, 50 per page, cursor pagination. Each row has old and new amount, version, reason, author and time. | 2 | Yes | LN-2.2 |
| LN-2.4 | As a report author, I want deletes blocked for values in use so that no report breaks. | Deleting a linked value returns 409 and lists the reports that use it. Deleting an unused value returns 204. | 2 |  | LN-3.3 |
| LN-2.5 | As a developer, I want concurrency tests so that the audit trail can be trusted. | Uses testcontainers Postgres. Two parallel PATCHes with the same version: exactly one succeeds, and the history gets exactly one row. Passes under `-race`. | 3 |  | LN-2.2 |

### LN-3 Reports and links

| ID | Story | Acceptance criteria | Pts | MVP | Depends on |
| --- | --- | --- | --- | --- | --- |
| LN-3.1 | As a report author, I want to create a report and save its cells so that I can build documents. | `POST /reports` and `PUT /reports/{id}/cells`. A `value_ref` must point to an existing value. Positions must be contiguous, starting at 0. Invalid input returns 422 naming the bad cell. | 3 | Yes | LN-2.1 |
| LN-3.2 | As a reader, I want a report with current numbers so that it is always right. | `GET /reports/{id}` returns each cell with its current amount and version, and sums computed with decimals. It uses one query, not one per cell. | 5 | Yes | LN-3.1 |
| LN-3.3 | As a preparer, I want to see where a value is used so that I know the impact of an edit. | `GET /values/{id}/usages` lists each report and cell position that links to the value. | 2 | Yes | LN-3.1 |
| LN-3.4 | As a report author, I want to choose each sum's inputs so that totals never depend on cell order. | Every sum cell lists its inputs in `report_cell_sum_inputs`, each with a sign (+1 or −1), so "Gross margin = Revenue − COGS" works. Inputs are value IDs, so cycles can't happen. `PUT /cells` returns 422 for a sum with no inputs. There is a test showing that reordering cells never changes a total. | 3 | Yes | LN-3.2 |

### LN-4 Frontend editor

| ID | Story | Acceptance criteria | Pts | MVP | Depends on |
| --- | --- | --- | --- | --- | --- |
| LN-4.1 | As a developer, I want a typed API client with a fake backend so that UI work can start before the services exist. | The Dart client covers every REST endpoint. An in-memory fake serves the seed data and can simulate a 409. A build flag switches between the fake and the real API. | 3 | Yes | LN-1.3 |
| LN-4.2 | As a developer, I want a Redux store so that state is predictable. | Slices: `values`, `reports`, `history`, `connection`. Selectors are memoized. Every reducer has unit tests. | 3 | Yes | LN-4.1 |
| LN-4.3 | As a preparer, I want a values grid so that I can edit numbers quickly. | Inline edit of the amount. Saving opens a dialog that requires a reason. Amounts are formatted by unit. On a 409 it shows "Changed by X at T" with the options Reload and Reapply my edit. | 5 | Yes | LN-4.2 |
| LN-4.4 | As a reader, I want a report view so that I can read a report with live numbers. | Linked cells show a chip with the value key. Hovering shows the last change. Sum rows are bold. Text cells render as plain text. | 5 | Yes | LN-4.2 |
| LN-4.5 | As a report author, I want a report builder so that I can make reports without SQL. | Add, reorder (drag or keyboard) and delete cells. Values are picked from a searchable combobox. Save calls `PUT /cells`, and validation errors show on the bad cell. For a sum cell, the author picks its input values and a + or − sign for each. | 5 | Yes | LN-4.4 |
| LN-4.6 | As a reviewer, I want a history panel so that I can audit a value. | Built as a `UiComponent2` class component. Shows a timeline of changes with old → new, reason, author and time, and lazy-loads older pages. | 3 | Yes | LN-4.3 |
| LN-4.7 | As a keyboard user, I want full keyboard access so that the app is accessible. | The edit + reason + save flow works with no mouse. Focus returns to the edited cell. axe checks pass in e2e. | 2 |  | LN-4.3 |

### LN-5 Real-time propagation

| ID | Story | Acceptance criteria | Pts | MVP | Depends on |
| --- | --- | --- | --- | --- | --- |
| LN-5.1 | As a developer, I want every committed edit to send a notification so that the gateway hears about each change. | values-api calls `pg_notify('values_changed', …)` inside the edit transaction, so a rolled-back edit sends nothing. The payload matches the event schema and stays under 8,000 bytes. | 2 | Yes | LN-2.2 |
| LN-5.2 | As a developer, I want events to list affected reports so that the gateway can route them. | The event payload includes `affectedReports`, taken from the usages query when the edit is written. | 2 | Yes | LN-3.3 |
| LN-5.3 | As a reader, I want a WebSocket gateway so that my open reports get updates. | The gateway keeps one dedicated pgx connection running `LISTEN values_changed`, and re-LISTENs after the connection drops. `/ws?reports=…` subscribes to those reports. Ping/pong every 30 s. A client whose send buffer fills is dropped with close code 1013. | 5 | Yes | LN-5.1 |
| LN-5.4 | As a reader, I want reports to update live so that I never see a stale number. | Reconnects with exponential backoff + jitter and refetches open reports on reconnect. Ignores events that aren't newer than the version it holds. A changed cell highlights for 1.5 s. | 5 | Yes | LN-5.3, LN-4.4 |
| LN-5.5 | As a developer, I want to measure propagation latency so that the 1-second target is proven. | Under k6 load of 20 edits/s, p95 time from PATCH to render in another tab is under 1 s. Measured with Playwright timestamps. | 3 |  | LN-5.4 |

### LN-6 Testing and quality

| ID | Story | Acceptance criteria | Pts | MVP | Depends on |
| --- | --- | --- | --- | --- | --- |
| LN-6.1 | As a developer, I want a written test plan so that coverage is deliberate. | `/docs/test-plan.md` covers what each layer tests, what is faked, and what runs in CI vs. nightly. | 2 |  | LN-1.2 |
| LN-6.2 | As a developer, I want component tests for the main UI so that refactors are safe. | Tests for the grid, report view and history panel using the React Testing Library bindings, including an async save and the 409 path. | 3 |  | LN-4.6 |
| LN-6.3 | As a developer, I want an end-to-end test of the demo so that it never breaks. | Playwright with two browser contexts: edit in one, see it in the other within 1 s, open history, run Explain against the fake LLM. Runs in CI. | 5 | Yes | LN-5.4 |
| LN-6.4 | As a developer, I want coverage gates so that quality doesn't slip. | CI fails if Go `internal/` coverage drops below 80% or Dart below 70%. | 2 |  | LN-6.2 |
| LN-6.5 | As a developer, I want a fault test so that I know the system recovers. | The gateway is killed mid-test. Edits still save. When it restarts, clients reconnect, refetch and show current numbers with no manual step. | 3 |  | LN-5.4 |

### LN-7 Observability and KPIs

| ID | Story | Acceptance criteria | Pts | MVP | Depends on |
| --- | --- | --- | --- | --- | --- |
| LN-7.1 | As an on-call engineer, I want one trace per edit so that I can follow it everywhere. | opentelemetry-dart in the browser and the OTel SDK in every Go service. Trace context rides in the notification payload. In Jaeger, one trace shows click → PATCH → SQL → NOTIFY → gateway → other tab. | 5 | Yes | LN-5.3 |
| LN-7.2 | As an on-call engineer, I want service metrics so that I can spot trouble early. | RED metrics per endpoint. Gauges for listener connection status and WebSocket connections. A histogram of propagation latency. All scraped by Prometheus. | 3 |  | LN-7.1 |
| LN-7.3 | As a team lead, I want a KPI dashboard so that we know the app is healthy. | Grafana panels and alerts for: PATCH p95 < 150 ms, propagation p95 < 1 s, error rate < 1%, listener reconnects < 3 per hour. The dashboard JSON is in the repo. | 3 |  | LN-7.2 |
| LN-7.4 | As an engineer, I want a debugging drill so that the team can rehearse incident response. | A feature flag adds 500 ms to one query. Find it using only traces and metrics, then write a blameless postmortem in `/docs/postmortems`. | 2 |  | LN-7.3 |

### LN-8 AI explain

| ID | Story | Acceptance criteria | Pts | MVP | Depends on |
| --- | --- | --- | --- | --- | --- |
| LN-8.1 | As a reviewer, I want an AI summary of a value's changes so that I understand the story quickly. | `explain-api` loads the history rows in the requested version range and asks the model for JSON `{summary, citations: [changeId]}`. The rows are the only facts the prompt contains. | 5 | Yes | LN-2.3 |
| LN-8.2 | As an auditor, I want every claim backed by a real row so that I can trust the summary. | The response is rejected if it cites an ID that wasn't provided, or if its summary has a number not in those rows. One retry, then 502. The model's raw output is logged with the trace ID. | 3 | Yes | LN-8.1 |
| LN-8.3 | As a developer, I want a fake LLM and an eval set so that AI tests run offline. | 10 recorded cases with expected citations, run in CI with no network. A report shows the pass rate. | 3 |  | LN-8.2 |
| LN-8.4 | As a reviewer, I want an Explain button in the history panel so that the summary is one click away. | Loading and error states. The summary is labeled "AI-generated". Each citation links to and highlights its history row. | 3 | Yes | LN-8.1, LN-4.6 |

### LN-9 Deploy and docs

| ID | Story | Acceptance criteria | Pts | MVP | Depends on |
| --- | --- | --- | --- | --- | --- |
| LN-9.1 | As a developer, I want container images for every service so that deploys are repeatable. | Multi-stage builds, distroless images, run as non-root. Built and tagged in CI. | 2 |  | LN-1.2 |
| LN-9.2 | As a developer, I want a local Kubernetes deploy so that it runs the way it would in production. | Kustomize manifests for kind: Deployments, Services, a migration Job, ConfigMaps, and a Secret for the LLM key. `make k8s-up` brings it all up. | 5 |  | LN-9.1 |
| LN-9.3 | As a new contributor, I want ADRs so that I can see how decisions were made. | At least 4 ADRs: LISTEN/NOTIFY vs. a message broker; NUMERIC + string decimals; WebSocket vs. SSE; the Redux state layout. | 3 |  | — |
| LN-9.4 | As any reader, I want a clear README so that I understand the project in 2 minutes. | Architecture diagram, quick start, demo script, and one paragraph for a finance manager. | 2 | Yes | LN-6.3 |
| LN-9.5 | As a stakeholder, I want a recorded demo so that I can see the app work without running it. | A recording of 5 minutes or less that follows the demo script in the README. | 1 | Yes | LN-9.4 |
| LN-9.6 | As a developer, I want an AWS deploy so that it runs on real cloud infrastructure. *(Stretch)* | Deployed on EKS or ECS with RDS Postgres. The README notes the monthly cost and a teardown command. | 5 |  | LN-9.2 |

## Non-functional requirements and definition of done

| Area | Requirement |
| --- | --- |
| Correctness | No floating-point math on amounts anywhere. Every edit adds exactly one history row. |
| Latency | PATCH p95 < 150 ms. Report GET p95 < 200 ms with 50 cells. Propagation p95 < 1 s. |
| Resilience | If the gateway is down, edits still save. Clients recover with no manual refresh. |
| Security | Parameterized SQL only (sqlc). No secrets in the repo. Containers run as non-root. The LLM key comes from a Secret. |
| Accessibility | The core flows work by keyboard, and axe reports no serious issues. |
| Observability | Every request has a trace ID in its logs, and every service exports RED metrics. |

**Definition of done, for every story**

- [ ] Acceptance criteria pass, with an automated test for each.
- [ ] Lint and format clean; CI green.
- [ ] The PR has a description, screenshots for UI changes, and a self-review. Aim for under 400 changed lines.
- [ ] New endpoints are in the OpenAPI spec; new decisions get an ADR.
- [ ] New code paths emit spans and structured logs.

## Open questions

- [x] Real-time transport: decided, Postgres LISTEN/NOTIFY (fewer moving parts; clients refetch on reconnect to cover missed notifications).
- [x] Subtotal rule: decided. LN-3.4 is in the MVP, so each sum lists its own inputs with a + or − sign, and moving cells never changes a total.
- [ ] Should some UI be built in TypeScript + React alongside Dart?
- [ ] Which LLM provider, and a monthly spending cap for the demo?
