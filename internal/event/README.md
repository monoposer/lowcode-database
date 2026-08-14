# Event types and EventBus

Type constants are used by `shared.EmitEvent` after row/schema writes.

- The bus publishes `records.*` and `schema.*` (`metadata.*` → `schema.*`).
- `EVENT_BUS=memory|redis`; webhooks: `POST /v1/admin/webhooks`.
- This service does not persist events; consumers record if they need a log.
