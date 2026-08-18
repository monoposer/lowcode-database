# schema module

**Path:** `internal/service/schema`  
**Role:** Logical table/column metadata (Meta DB) and Data DB physical DDL; virtual column config and validation.

## Core files

| File | Role |
|------|------|
| `table.go` | CreateTable / DeleteTable / Rename / List |
| `column_mutate.go` | AddColumn / UpdateColumn; resolve typeId (pgType / columnType) |
| `column_crud.go` | ListColumns, type-change DDL |
| `column_virtual.go` | link / lookup / rollup config normalization |
| `virtual.go` | Virtual column SQL expansion (ListRows, etc.) |
| `formula.go` | Formula expression validation |
| `fk.go` | Foreign-key constraint DDL |
| `table_name.go` | Table PK `idType` (uuid / bigint) |
| `result_type.go` | formula/lookup/rollup result type inference |
| `loaders.go` | Table-level schema load |
| `relation.go` | `/v1/admin/relations`: project Link columns as Relation |

## Meta tables

- `lc_tables` — `name` (= public Id), `vt_id`, `label`
- `lc_columns` — `type_id`, `config` JSONB, `position`

Logical tables do not get a physical `CREATE TABLE`. Rows live in `record` (filter by `vt_id`).

## Virtual columns (no physical column)

| typeId | Config |
|--------|--------|
| `formula` | `expression` |
| `link` | `target_table_name` (+ optional `link_column_id` \| `target_column_id` for FK-style); Teable two-way: `bidirectional` (default true for `link_ref`), `inverse_field_name`, `inverse_cardinality` |
| `lookup` / `rollup` | `relation_column_id` + … |

Adding or updating formula / lookup / rollup enqueues existing rows onto `calc_queue` so `record.data` cache is backfilled. Link writes enqueue the edited row and related records.

See [roadmap](../roadmap.md) for schema bundle import (not in this version).

## Admin API

`POST /v1/admin/tables`, `/columns`, `/indexes` (indexes delegated to catalog)

## Dependencies

- `catalog` — type resolution, index introspection
- `shared` — Base, EmitEvent, ValidateColumnName
- `internal/event` — schema change events
