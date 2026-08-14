# event module

**Path:** `internal/event`

## Role

- Event type constants: `records.*` (row writes) and `metadata.*` (mapped to `schema.*` on the bus).
- **EventBus**: `EmitEvent` publishes `records.*` and `schema.*`.
- Implementations:
  - `memory` — single process (`EVENT_BUS=memory`, default)
  - `redis` — Redis Stream (`EVENT_BUS=redis` + `REDIS_URL`) for multi-instance consumers
- **Webhooks**: `POST /v1/admin/webhooks`; dispatcher POSTs matching events. Header `X-Webhook-Signature: sha256=...` when a secret is set.

This service does not persist an event log. External systems record via webhook (or a bus consumer).
