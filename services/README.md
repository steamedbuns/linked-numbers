# services

One Go module with one `cmd/<name>` per binary:

| Binary | Role |
| --- | --- |
| `values-api` | REST API; the only service that reads or writes the tables |
| `realtime-gateway` | LISTENs on `values_changed` and pushes events over WebSocket |
| `explain-api` | AI change summaries that cite history rows |

The shared service template arrives in LN-1.4, and migrations, seed data and sqlc in LN-1.5.
