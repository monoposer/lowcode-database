# event module

**Path:** `internal/event`

## Role

- Event type constants: `records.*` (row writes) and `metadata.*` (schema audit names).
- **EventBus**: `EmitEvent` publishes `records.*` and `schema.*` (`metadata.*` is mapped to `schema.*` on the bus).
- Implementations:
  - `memory` — single process (`EVENT_BUS=memory`, default)
  - `redis` — Redis Stream (`EVENT_BUS=redis` + `REDIS_URL`) for multi-instance consumers
- **Webhooks**: `POST /v1/admin/webhooks`; dispatcher POSTs matching events. Header `X-Webhook-Signature: sha256=...` when a secret is set.

`lc_schema_audit` still records `metadata.*`. External systems should subscribe instead of polling the audit table.
