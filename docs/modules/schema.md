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
| `table_id.go` | Table PK `idType` (uuid / bigint) |
| `result_type.go` | formula/lookup/rollup result type inference |
| `loaders.go` | Table-level schema load |
| `relation.go` | `/v1/admin/relations`: project Link columns as Relation |

## Meta tables

- `lc_tables` — `name` (= public Id), `schema_name`, `label`
- `lc_columns` — `type_id`, `config` JSONB, `position`

## Physical DDL

Non-virtual columns, non-`rls_table` mode: `ALTER TABLE schema.table ADD COLUMN …`

Column type SQL comes from **catalog**:

- built-in → `columntype` + `EffectivePgType`
- domain → `"schema"."domain_name"`

## Virtual columns (no physical column)

| typeId | Config |
|--------|--------|
| `formula` | `expression` |
| `link` | `target_table_id` + `link_column_id` \| `target_column_id` |
| `lookup` / `rollup` | `relation_column_id` + … |

See [roadmap](../roadmap.md) for schema bundle import (not in this version).

## Admin API

`POST /v1/admin/tables`, `/columns`, `/indexes` (indexes delegated to catalog)

## Dependencies

- `catalog` — type resolution, index introspection
- `shared` — Base, EmitEvent, ValidateColumnName
- `internal/event` — schema change events
