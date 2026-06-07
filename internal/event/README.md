# Event types and EventBus

Type constants are used by `shared.EmitEvent` after row/schema writes.

- `metadata.*` is recorded in `lc_schema_audit`.
- The bus publishes `records.*` and `schema.*` (`metadata.*` → `schema.*`).
- `EVENT_BUS=memory|redis`; webhooks: `POST /v1/admin/webhooks`.
