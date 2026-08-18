# catalog module

**Path:** `internal/service/catalog` + `internal/columntype`  
**Role:** Column type system, tenant columnTypes, index catalog, column metadata loaders.

## Type system

```
typeId resolution order: pgType → columnType
```

| Kind | Source | Storage |
|------|--------|---------|
| **pgType** | `internal/columntype` built-in registry | No meta row |
| **columnType** | Admin `POST /column-types` | Meta `lc_column_types` + Data `CREATE DOMAIN` |

## Core files

| File | Role |
|------|------|
| `types.go` | `ListTypes` — pgType + tenant columnTypes |
| `column_type.go` | columnType CRUD, `ImportTypeCatalog` (types portion) |
| `loaders.go` | `LoadColumns`, `LoadAllColumnMeta` |
| `pg_index.go` | Read indexes from `pg_catalog` |
| `index.go` | CreateIndex / DeleteIndex (`lc_indexes` + PG DDL) |
| `index_meta.go` | `lc_indexes` CRUD, backfill |

## columntype (built-in pgType + columnType spec)

**Path:** `internal/columntype`

Canonical: `text`, `number`, `datetime`, `boolean`, `jsonb`, virtual kinds `formula` / `link` / `lookup` / `rollup`.

**Arrays** are a property of tenant **columnTypes** (`spec.array`), not column `config`. `spec.pgType` must be one of `text` | `number` | `datetime` | `boolean` | `jsonb`. Create separate types for SELECT vs MULTI_SELECT:

```http
POST /v1/admin/column-types
{ "name": "select", "label": "Select", "spec": { "pgType": "text" } }

POST /v1/admin/column-types
{ "name": "multi_select", "label": "Multi Select", "spec": { "pgType": "text", "array": true } }

POST /v1/admin/columns
{ "tableName": "orders", "name": "tags", "typeId": "multi_select" }
```

| | `select` | `multi_select` |
|--|----------|----------------|
| Spec | `{ "pgType": "text" }` | `{ "pgType": "text", "array": true }` |
| Logical PG | `text` | `text[]` |
| VR `record.data` | JSON scalar | JSON array `["a","b"]` |
| `GET /types` | — | `config.array: true`, `pgType: "text[]"` |

Tenant columnTypes are stored in `lc_column_types` (`POST /v1/admin/column-types`). Do **not** use `{ "typeId": "text", "config": { "array": true } }`.

## Admin API

| Path | Module method |
|------|---------------|
| `GET /v1/admin/types` | `Catalog.ListTypes` |
| `/v1/admin/column-types` | `Catalog.*ColumnType*` |
| `/v1/admin/indexes` | `Catalog.*Index*` |
| `/v1/admin/indexes:backfill` | `Catalog.BackfillIndexesFromPG` |

## Notes

- **Index** meta table `lc_indexes`; PG catalog is the physical DDL source of truth.
- Other domains call loaders via `catalog.New(base)` (`schema`/`meta`); the same `Base` is stateless, equivalent to the embedded `Catalog` instance.
