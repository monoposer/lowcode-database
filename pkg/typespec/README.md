# typespec — Column Type Contract

Portable **column type contract** for lowcode-database: built-in **pgTypes**, tenant **columnTypes** (Postgres DOMAIN validation).

Go import:

```go
import "github.com/monoposer/lowcode-database/pkg/typespec"

sql, err := typespec.CreateColumnTypeSQL("tenant_acme", "positive_price", spec)
```

JSON Schema files are not shipped. Admin API (runtime): `GET/POST /v1/admin/column-types`, `GET /v1/admin/types` (pgType + columnType).

---

## Type resolution (column `typeId`)

| `typeId` | Kind | Storage |
|----------|------|---------|
| `text`, `numeric`, … | **pgType** | Built-in PG type |
| `positive_price` | **columnType** | PG DOMAIN in tenant schema |
| `formula`, … | **virtual** | Meta only, no physical column |

Resolution order: **pgType → columnType**. Names must not collide with built-in pgType ids.

---

## pgTypes (built-in)

| id | pgType | notes |
|----|--------|-------|
| `text` | text | optional **array** |
| `number` | numeric | optional precision/scale and **array** |
| `datetime` | timestamptz | optional **array** |
| `boolean` | boolean | optional **array** |
| `jsonb` | jsonb | optional **array** |
| `point` | geometry(Point,4326) | PostGIS; optional **array** |

Array is a **modifier** (`config.array: true`), not a separate type id. Legacy ids (`numeric`, `bigint`, `timestamptz`, `text_array`, `relationship`, …) still resolve for existing columns.

Virtual: `formula`, `link`, `lookup`, `rollup`.

---

## ColumnType (tenant custom validation)

PostgreSQL DOMAIN: one underlying pgType + CHECK / DEFAULT / NOT NULL.

```json
{
  "apiVersion": "typespec.lowcode/v1",
  "kind": "ColumnType",
  "metadata": {
    "name": "positive_price",
    "label": "Positive amount"
  },
  "spec": {
    "pgType": "number",
    "precision": 18,
    "scale": 2,
    "checks": [{ "expr": "VALUE > 0" }]
  }
}
```

Create column: `{ "typeId": "positive_price", ... }` → DDL uses `"tenant_xxx"."positive_price"`.

### Array columnTypes

✅ `pgType: "text[]"` with CHECK on `VALUE` / `unnest(VALUE)`  
❌ `positive[]` where `positive` is a columnType — PG forbids domain-of-domain arrays; put `[]` on `pgType`.

---

## TypeCatalog (init / import)

```json
{
  "apiVersion": "typespec.lowcode/v1",
  "kind": "TypeCatalog",
  "columnTypes": [ { "...": "ColumnType" } ]
}
```

`POST /v1/admin/column-types:import` accepts this bundle.

---

## Meta table

`lc_column_types` (meta DB): `tenant_id`, `name`, `schema_name`, `spec` JSONB, timestamps.

Physical DOMAIN created in tenant data DB on create/update; dropped on delete.

See [docs/roadmap.md](../../docs/roadmap.md) for plugin types and schema-bundle import (not in this version).
