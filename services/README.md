# services

One Go module with one `cmd/<name>` per binary:

| Binary | Role |
| --- | --- |
| `values-api` | REST API; the only service that reads or writes the tables |
| `realtime-gateway` | LISTENs on `values_changed` and pushes events over WebSocket |
| `explain-api` | AI change summaries that cite history rows |

For now the module holds only `internal/buildinfo`, so CI has something to check (LN-1.2). The shared service template arrives in LN-1.4, and migrations, seed data and sqlc in LN-1.5.
