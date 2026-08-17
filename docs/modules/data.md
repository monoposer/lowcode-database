# data module

**Path:** `internal/service/data`  
**Role:** Row I/O (`record` jsonb), DSL queries, `link_ref`, calc_queue enqueue, bulk, export.

## Core files

| File | Role |
|------|------|
| `vr_rows.go` | Create/Update/Delete/Get/list (`record` + version + enqueue) |
| `link_write.go` / `record_cells.go` | `link_ref` binding; cache field hydrate |
| `query.go` / `query_exec.go` | ListRows, `:query`, SearchRows |
| `vr_fulltext.go` / `cascade.go` | Full text |
| `bulk.go` | Bulk upsert/delete |

Calc engine: `internal/service/calc` (queue + in-process worker). Design: [record-calc.md](../architecture/record-calc.md).

## Data flow

```
/v1/data/tables/{id}/rows
  → meta.LoadColumns (catalog) + TableVTID
  → SQL on record or `{tenant_id}_record` (DataReadPool for query/export; DataPool for writes)
  → EmitEvent → EventBus (records.* / schema.*) → webhooks
```

## Query

- Filter / sort: `internal/dsl` + VR JSONB predicates (`data->>` / FTS / `in_record_ids`); lookup/formula/rollup filters read `record.data` calc cache
- Slow queries: `SLOW_QUERY_THRESHOLD_MS` warn
- SQL statements: debug (`LOG_LEVEL=debug`)

## Admin vs Data

| Operation | Plane |
|-----------|-------|
| Change table structure | `/v1/admin/*` → schema |
| Read/write rows | `/v1/data/*` → data |

## Dependencies

- `meta` / `catalog` — column metadata, `vt_id`
- `shared.EmitEvent`
