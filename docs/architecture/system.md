# System architecture (current)

This document describes the **current code path**: processes, dual database, tenant shards, `record` storage, async calc, the query stack, and **pgx multi-tenant access**. Tenant routing and `record_store`: [tenant-isolation.md](tenant-isolation.md).

Related: [analysis.md](analysis.md) · [virtual-records.md](virtual-records.md) · [record-calc.md](record-calc.md) · [tenant-isolation.md](tenant-isolation.md)

---

## 1. Product

lowcode-database is a **multi-tenant low-code table service**: HTTP JSON (`/v1/*`), runtime-defined tables/columns/relations, rows stored on a PostgreSQL shard per tenant. Public contracts live in `internal/service/{schema,catalog,data,platform,shared}/` (no gRPC / protobuf).

| Plane | Prefix | Role |
|-------|--------|------|
| Admin / Meta | `/v1/admin/*` | Tenant, Base, tables/columns, indexes, relations, Query, API Key |
| Data | `/v1/data/*` | Row CRUD, queries, execute saved Query |
| Observability | `/v1/admin/pg-stat-statements`, `/openapi`, `/swagger` | SQL stats, OpenAPI |

| Header | Role |
|--------|------|
| `X-Tenant-Id` | Tenant (`tenant_id`) |
| `X-Base-Id` | Base; defaults to the tenant default Base |
| `X-Api-Key` | Optional authentication |

---

## 2. Processes

```
┌──────────────── cmd/server (single instance) ─────────┐
│  CORS → log → authn → /v1/admin + /v1/data               │
│  calc worker polls calc_queue                            │
│  EventBus + webhook dispatcher                           │
│  background: IndexMigrate                                │
└────────────────────────────────────────────────────────┘
                     │
              web/playground  (:5173 → :8080)
```

| Entry | Role |
|-------|------|
| `cmd/server` | Only runtime: admin + data + calc on `HTTP_ADDR` (default `:8080`) |
| `cmd/migrate` | Apply `migrations/` only; **the server does not auto-migrate** |
| `web/playground` | Debug UI |

Local: `make run` (`:8080`).

---

## 3. Layers

```
HTTP JSON  (internal/api + service domain types)
        │
LowcodeService facade
  schema │ catalog │ data │ platform │ calc
        │
shared.Base  (TenantManager · Cache · Metrics)
        │
┌───────┴────────┐
│ pgx + Where    │  static CRUD (lc_* / record) must include tenant_id / base_id
│ pgx dynamic SQL│  DSL filters, FOR UPDATE SKIP LOCKED
└───────┬────────┘
        │
 Meta DB (shared)          Data DB (per-tenant tenants.data_dsn)
```

Handlers do JSON and validation only. Domain logic lives in `internal/service/{domain}`. Connection routing lives in `pkg/infra/postgres`.

```go
type LowcodeService struct {
    *schema.Schema
    *catalog.Catalog
    *data.Data
    *platform.Platform
}
```

Dependencies (pools, cache, logger) are injected via `shared.Base`.

Middleware (`cmd/server`): CORS → request log → optional `authn.Validator` (`X-Api-Key`) → chi `NewHandler`.

---

## 4. Dual database and tenant routing

### 4.1 Databases

| DB | Connection | Contents |
|----|------------|----------|
| **Meta** | `META_DATABASE_URL` | `tenants` (`data_dsn`), `lc_bases`, `lc_tables` / `lc_columns` / `lc_indexes` / `lc_queries`, API keys |
| **Data** | `tenants.data_dsn` | `record` or `{tenant_id}_record` when `record_store=dedicated`; `link_ref`, `calc_queue` |

`TenantManager` does **not** run migrations at startup. Create tenants and bases via Admin API (`POST /tenants`, `POST /bases`).

Public `Table.Id` is the logical **name**, not a UUID. Physical column names match logical `name`. Virtual columns have no physical column.

### 4.2 Hierarchy

```
Tenant (tenant_id)  ──data_dsn──►  Data DB
    └── Base (base_id)
            └── Table (name = API Table.Id)
                    └── Column (logical name; virtual columns have no physical column)
```

Row key `vt_id` is globally unique (`lc_tables.vt_id`). Multiple tenants may share one data DSN; rows and meta always carry `tenant_id`. Set `tenants.record_store=dedicated` for a private `{tenant_id}_record` table on that DSN; dedicated Postgres is still `data_dsn`.

### 4.3 Pools

The process does **not** keep a single “business DB” connection. Meta and Data each use a `pgxpool.Pool`. Data pools are keyed by **DSN** (shared across tenants on the same shard), not by `tenant_id`.

```
TenantManager
  metaPool        ← META_DATABASE_URL (created at startup)
  dataPools (map) ← lazy per write/read DSN
```

| Pool | API | Use |
|------|-----|-----|
| **MetaPool** | `TenantManager.MetaPool()` | All `lc_*` / tenants / API keys / webhooks |
| **DataPool(ctx)** | `TenantManager.DataPool(ctx)` | Write DSN (`data_dsn` / `data_dsn_write`) |
| **DataReadPool(ctx)** | `TenantManager.DataReadPool(ctx)` | Replica list, or primary if `consistency=strong` |

Implementation: `pkg/infra/postgres` (`tenant_manager.go`, `shard_pool.go`, `tenant_pool.go`, `scope.go`).

```
HTTP + X-Tenant-Id
    → tenant.ResolveTenantID(ctx)
    → Meta: tenants.data_dsn
    → DataPool(ctx)
```

- Meta and Data are **always separate pools**, even if they happen to be the same Postgres instance.
- Static SQL must include tenant predicates (`postgres.Where` / `AndWhere`): meta `tenant_id` + `base_id`; `record` has `tenant_id` and `vt_id`.
- Worker scans of `calc_queue` may have no tenant in context; do not add row filters then.

| Mechanism | Behavior |
|-----------|----------|
| Lazy create | Pool created on first `DataPool` hit for a tenant |
| LRU eviction | `MAX_TENANT_DATA_POOLS` (default 50) closes least-recently used pools |
| Observability | `TenantManager.ActiveDataPoolCount()` |

| Variable | Default | Scope |
|----------|---------|--------|
| `PG_MAX_CONNS` | 10 | `MaxConns` per meta / data pool |
| `PG_MIN_CONNS` | 1 | `MinConns` per pool |
| `PG_MAX_CONN_LIFETIME_MIN` | 60 | Connection max lifetime (minutes) |
| `MAX_TENANT_DATA_POOLS` | 50 | Max cached data pools in-process |

See [tenant-isolation.md](tenant-isolation.md).

---

## 5. Rows and calc

Authoritative detail: [virtual-records.md](virtual-records.md), [record-calc.md](record-calc.md).

| Object | Location | Notes |
|--------|----------|-------|
| User fields + calc cache | `record.data` jsonb | Does not store link id arrays |
| Link edges | `link_ref` | Bidirectional → two rows |
| Calc tasks | `calc_queue` | Authoritative queue; MQ may wake workers, no payload |

1. Change a scalar field → update `record.data` + `version` → enqueue downstream.
2. Change a Link → write `link_ref` only → enqueue this record.
3. Remote change → reverse-lookup `from_record_id` and fan-out enqueue.
4. Worker `SELECT … FOR UPDATE SKIP LOCKED` → write cache keys + optimistic `version`.

List reads the cache; detail may compute the DAG live. **No cross-row strong consistency.**

`internal/service/calc.Worker` always runs in-process inside `cmd/server`. Calc is not exposed over HTTP.

---

## 6. Query stack

```
QueryRows / ExecuteQuery
    → merge saved Query filter/sort/column projection
    → dsl.Parse → query.BuildWhere (jsonb paths)
    → executeVRQuery: SELECT record … WHERE vt_id AND tenant_id AND (DSL)
```

Dynamic WHERE / sort / paging use **pgx-assembled SQL** (column sets are known only at runtime). Table/column/single-row CRUD also uses pgx with `postgres.Where` / `AndWhere`.

Index = PG physical index + `lc_indexes`. columnType = `lc_column_types` + `internal/columntype`.

Virtual column kinds: `formula`, `link`, `lookup`, `rollup` — no standalone physical columns.

---

## 7. pgx and tenancy

Schema comes from `migrations/` and `EnsureVirtualRecordsParent`. Access layer is **pgx only** (`MetaPool` / `DataPool`).

HTTP puts `tenant_id` / `base_id` on context. Static SQL **must** include tenant predicates:

- meta tables: `tenant_id` + `base_id`
- `record`: `tenant_id` and `vt_id`
- `link_ref` / `calc_queue`: column `tenant_id`
- **No filter when context has no tenant** (worker scans the whole shard `calc_queue`)

`lc_tables` physical PK is `(tenant_id, base_id, name)`. Never look up by `name` alone.

| Path | Why dynamic SQL |
|------|-----------------|
| `EnsureVirtualRecordsParent` | Shared or dedicated `record` / `link_ref` / `calc_queue` |
| `executeVRQuery` and DSL | Runtime columns → jsonb expressions |
| `calc.Claim` SKIP LOCKED | Queue claim |
| pg_catalog INDEX | Catalog is source of truth |

---

## 8. Cross-cutting

| Capability | Status |
|------------|--------|
| Events | EventBus + webhooks |
| Cache | Redis: Query / column spec |
| Metrics | `PG_STAT_STATEMENTS=true` → `GET /v1/admin/pg-stat-statements` |
| Authn | Optional API Key |

Plugins, graph expand, Choice/ENUM, RBAC, schema-bundle import: [roadmap](../roadmap.md).

| Capability | Config | Notes |
|------------|--------|-------|
| Metadata cache | `REDIS_URL` + `CACHE_ENABLED` | Invalidate on writes |
| SQL stats | `PG_STAT_STATEMENTS=true` | Postgres `pg_stat_statements` |
| Slow query | `SLOW_QUERY_THRESHOLD_MS` | query / SQL warn logs |
| SQL log | `LOG_LEVEL=debug` | pgx SQL + args |
| Tracing | `pkg/telemetry` | OpenTelemetry; OTLP when `OTEL_EXPORTER_OTLP_ENDPOINT` is set |

---

## 9. API surfaces

Routes: `internal/api/routes.go` (chi). Tests and `cmd/server` mount all prefixes via `NewHandler`. JSON fields are **camelCase**. OpenAPI: `internal/api/openapi/openapi.yaml`; Swagger UI: `/swagger/`.

### Admin

| Domain | Routes |
|--------|--------|
| Tenant / Base | `GET/POST /v1/admin/tenants`; `GET/POST /v1/admin/bases` |
| Table / Column | `GET/POST /v1/admin/tables`, `GET .../schema`, `POST ...:rename`; `GET/POST/PATCH/DELETE /v1/admin/columns` |
| Index | `GET/POST/DELETE /v1/admin/indexes`, `POST /v1/admin/indexes:backfill` |
| ColumnType | `GET/POST/PATCH/DELETE /v1/admin/column-types`, `POST /v1/admin/column-types:import` |
| Relation | `GET/POST/DELETE /v1/admin/relations` |
| Query | `GET/POST/PATCH/DELETE /v1/admin/queries` |
| ER | `GET /v1/admin/schema/er` |
| Platform | API keys, types, webhooks, `pg-stat-statements` |

### Data

| Method | Path | Notes |
|--------|------|-------|
| GET/POST/PATCH/DELETE | `/v1/data/tables/{tableName}/rows[/{rowId}]` | Row CRUD |
| POST | `.../rows:query` | DSL filter query |
| POST | `.../rows:bulkUpsert` / `:bulkDelete` / `:export` / `:search` | Bulk and I/O |
| POST | `/v1/data/queries/{name}` | Execute a saved Query |

### Write path

| Write | HTTP | Event | Tx |
|-------|------|-------|-----|
| CreateRow | `POST .../rows` | records.after.insert | data pool tx |
| UpdateRow | `PATCH .../rows/{id}` | records.after.update | same |
| BulkUpsert | `POST .../rows:bulkUpsert` | per row | single-table tx |
| Schema change | Admin API | schema.* (EventBus / webhooks) | meta + data DDL |

---

## 10. Directory map

| Path | Role |
|------|------|
| `cmd/server` `cmd/migrate` | Runtime + migration CLI |
| `internal/api` | chi routes and handlers |
| `internal/service/*` types | JSON resource / request types |
| `internal/service/*` | Domain logic |
| `pkg/infra/postgres` | Pools, shard routing, `Where` helpers |
| `pkg/config` `pkg/logger` `pkg/tenant` | Env, logs, tenant context |
| `internal/service/calc` | Queue engine + in-process `calc_queue` worker |
| `internal/columntype` | pgType registry and columnType spec |
| `web/playground` | Debug UI |
| `migrations/` | Meta/Data SQL |

Module index: [modules/README.md](../modules/README.md). Out of scope: [roadmap.md](../roadmap.md).

---

## 11. Row write lifecycle

```
POST /v1/data/tables/{name}/rows
  + X-Tenant-Id + X-Base-Id
        │
        ▼
  Resolve tenant → shard DataPool
  LoadColumns (MetaPool, WHERE tenant_id/base_id)
  INSERT record (pgx; tenant_id + vt_id + data)
  persist link_ref
  Enqueue calc_queue
  EmitEvent
        │
        ▼
  calc.Worker (in-process)
  SKIP LOCKED → formula/lookup/rollup → optimistic write data
```

---

## 12. Core env vars

| Variable | Role |
|----------|------|
| `META_DATABASE_URL` | Meta DB |
| `DATA_DATABASE_URL` | Optional local default for tenant `data_dsn` (not used by migrate) |
| `HTTP_ADDR` | Listen address (default `:8080`) |
| `MAX_ROW` | Max rows per query |
| `REDIS_URL` / `CACHE_ENABLED` / `CACHE_TTL_SECONDS` | Metadata cache |
| `PG_STAT_STATEMENTS` | Enable `GET /v1/admin/pg-stat-statements` |
| `API_KEY_REQUIRED` | Require API Key |
| `PG_MAX_CONNS` / `PG_MIN_CONNS` / `PG_MAX_CONN_LIFETIME_MIN` | pgx pools |
| `MAX_TENANT_DATA_POOLS` | Data-pool LRU cap (default 50) |
| `SLOW_QUERY_THRESHOLD_MS` / `LOG_LEVEL` | Logging |

Full list: [`.env.example`](../../.env.example).

### Migration

| Target | Location | Notes |
|--------|----------|-------|
| Meta | `migrations/meta/*.up.sql` | Schema only (no tenant/base DML). `cmd/migrate` / `make migrate` |

Data tables, indexes, PostGIS, and `pg_stat_statements` are created by **runtime DDL** (`EnsureDataTables`). SQL is idempotent (`IF NOT EXISTS`). **`cmd/migrate` does not touch data databases.**

---

## 13. Local debug

```bash
make docker-up
make migrate
make run                          # API :8080, default embed worker
make playground-dev               # UI, see web/playground
```
