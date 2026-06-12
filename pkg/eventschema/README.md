# eventschema — Event Delivery JSON Schema

**[English](README.md) | [中文](README.zh.md)**

Importable **lowcode-database event delivery contract**: a shared envelope wrapper plus per-`type` `data` payloads.

Go import:

```go
import "github.com/solat/lowcode-database/pkg/eventschema"

raw := eventschema.EnvelopeSchema()
payload, ok := eventschema.PayloadSchema(eventschema.RecordsAfterInsert)
```

Non-Go consumers: copy JSON Schema files from [`schema/`](schema/).

Runtime discovery (when the service is running):

- `GET /v1/admin/events/schemas` — all payload schemas
- `GET /v1/admin/events/envelope-schema` — envelope schema

---

## Envelope

Every message delivered via HTTP POST to a webhook `targetUrl` shares the same outer shape:

```json
{
  "type": "records.after.insert",
  "tenantId": "default",
  "tableId": "orders",
  "occurredAt": "2026-06-12T10:00:00.123456789Z",
  "data": { }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `type` | yes | Event type (see categories below) |
| `tenantId` | yes | Tenant id (`X-Tenant-Id`) |
| `tableId` | no | Logical table name when applicable |
| `occurredAt` | yes | UTC timestamp, RFC3339Nano |
| `data` | yes | Type-specific payload; see JSON Schema per type |

Schema file: [`schema/envelope.json`](schema/envelope.json)

---

## Categories

### 1. Data events (`records.*`)

**Tenant data row changes** — use for search indexes, caches, downstream sync, etc.

| type | When | data |
|------|------|------|
| `records.after.insert` | Single row created | `{ "row": { ... } }` |
| `records.after.update` | Single row updated | `{ "row": { ... } }` |
| `records.after.delete` | Single row deleted | `{ "rowId": "..." }` |
| `records.after.bulkUpsert` | Bulk upsert | `{ "rows": [ ... ] }` |
| `records.after.bulkDelete` | Bulk delete | `{ "rowIds": [ ... ] }` |
| `records.after.bulkImport` | Import | `{ "rows": [ ... ], "insertedCount": N }` |

`row` / `rows` keys are column logical names or ids; cell values use the API `Value` union (`stringValue`, `numberValue`, …).

Example:

```json
{
  "type": "records.after.insert",
  "tenantId": "default",
  "tableId": "orders",
  "occurredAt": "2026-06-12T10:00:00Z",
  "data": {
    "row": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "amount": { "numberValue": 99.5 },
      "status": { "stringValue": "pending" }
    }
  }
}
```

### 2. Metadata events (`metadata.*`)

**Meta schema changes** (tables, columns, enums, indexes, relations, data sources) — use to invalidate caches, rebuild ER diagrams, trigger CI, etc.

| type | When | data |
|------|------|------|
| `metadata.table.created` | Table created | `{ "table": { ... } }` |
| `metadata.table.deleted` | Table deleted | `{ "tableId": "..." }` |
| `metadata.table.renamed` | Table renamed | `{ "oldName", "newName", "table" }` |
| `metadata.column.created` | Column added | `{ "column": { ... } }` |
| `metadata.column.updated` | Column updated | `{ "column": { ... } }` |
| `metadata.column.deleted` | Column removed | `{ "tableId", "columnId" }` |
| `metadata.choice.created` | ENUM registered | `{ "choice": { ... } }` |
| `metadata.choice.updated` | ENUM updated | `{ "choice": { ... } }` |
| `metadata.choice.deleted` | ENUM removed | `{ "choiceId": "..." }` |
| `metadata.relation.created` | Relation created | `{ "relation": { ... } }` |
| `metadata.relation.deleted` | Relation removed | `{ "relationId": "..." }` |
| `metadata.index.created` | Index created | `{ "index": { ... } }` |
| `metadata.index.deleted` | Index dropped | `{ "tableId", "pgIndexName" }` |
| `metadata.datasource.created` | DataSource created | `{ "dataSource": { ... } }` |
| `metadata.datasource.updated` | DataSource updated | `{ "dataSource": { ... } }` |
| `metadata.datasource.deleted` | DataSource removed | `{ "tableId", "name" }` |

Nested resources (`table`, `column`, …) match Admin API JSON (camelCase).

---

## Layout

```
pkg/eventschema/
├── README.md / README.zh.md
├── types.go       # constants, categories, TypeInfo
├── envelope.go    # Envelope struct
├── schema.go      # embed loader API
└── schema/
    ├── envelope.json
    ├── records/
    │   └── after.*.json
    └── metadata/
        └── *.json
```

---

## Versioning

- JSON Schema draft: **2020-12**
- New event types: add `schema/<category>/<suffix>.json` (e.g. `schema/records/after.insert.json` for `records.after.insert`), register in `types.go`, keep the same `type` string in `internal/event`
- Non-breaking: extend `data` fields only; removals require a major version note
