# catalog module

**Path:** `internal/service/catalog` + `internal/columntype` + `pkg/typespec`  
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

## columntype (built-in pgType)

**Path:** `internal/columntype/types.go`

Canonical: `text`, `numeric`, `bigint`, `float8`, `boolean`, `timestamptz`, `date`, `uuid`, `jsonb`, `bytea`, PostGIS, virtual kinds such as `formula`.

Legacy aliases (`int8`, `number`, `text_array`…) still `Resolve`, but are omitted from `List()`.

## pkg/typespec

Go helpers for pgType / DOMAIN — [pkg/typespec](../../pkg/typespec/README.md).

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
