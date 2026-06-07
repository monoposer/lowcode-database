# Local development (Cursor)

## Environment

The code uses a **dual-DB model** (not the old `TENANT_MODE` in stale READMEs):

```bash
# Meta: all lc_* metadata + tenants
META_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/lowcode_meta

# Default tenant data DB (record / link_ref / calc_queue)
DEFAULT_TENANT_DATA_DSN=postgresql://postgres:postgres@localhost:5432/lowcode_data
DEFAULT_TENANT_ID=default

HTTP_ADDR=:8080
MAX_ROW=100

# Optional: Redis metadata cache
# REDIS_URL=redis://localhost:6379/0
# CACHE_ENABLED=true
# CACHE_TTL_SECONDS=300
# PG_STAT_STATEMENTS=true      # GET /v1/admin/pg-stat-statements
# LOG_LEVEL=info
# SLOW_QUERY_THRESHOLD_MS=500

# Optional: CREATE DATABASE when Admin creates a tenant
# DATA_ADMIN_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/postgres
# DATA_DSN_TEMPLATE=postgresql://postgres:postgres@localhost:5432/%s
```

Copy: `cp .env.example .env` and edit as above.

## Run and debug

```bash
make docker-up      # postgis/postgis:16-3.5; empty DBs lowcode_meta / lowcode_data
make migrate        # or make docker-migrate / go run ./cmd/migrate -target all
make run            # HTTP service (does not migrate)
make test
make test-integration
```

Playground UI: in-repo `web/playground` (`make playground-dev`).

Integration tests:

```bash
export TEST_META_DATABASE_URL='postgresql://postgres:postgres@localhost:5432/lowcode_meta'
export TEST_DATA_DATABASE_URL='postgresql://postgres:postgres@localhost:5432/lowcode_data'
make test-integration
```

HTTP examples:

```bash
curl -H 'X-Tenant-Id: default' http://localhost:8080/v1/admin/tables
curl -H 'X-Tenant-Id: default' http://localhost:8080/v1/admin/schema/er
```

## Dual-DB roles

```
HTTP /v1/*
    → internal/service.LowcodeService
    → MetaPool (META_DATABASE_URL)     — table/column/query registration
    → DataPool (tenants.data_dsn)      — record rows, indexes
```

- Startup `db.NewTenantManager` does **not** run migrations
- Schema source: `migrations/` + `cmd/migrate` (compose **does not** auto-apply)

## Migration

SQL: `migrations/meta/`; data: [data/README.md](../migrations/data/README.md) (PostGIS `000001_postgis.up.sql`).

| Command | Notes |
|---------|-------|
| `make docker-up` | postgres + redis only; empty volume init only CREATE DATABASE |
| `make migrate` | `cmd/migrate` apply meta + data (embed `migrations/`) |
| `make docker-migrate` | compose `run --rm migrate` (not started by docker-up) |
| `go run ./cmd/migrate -target meta -database-url '...'` | Single-DB migrate |

**Data DB:** business tables still use runtime DDL. UUID PKs use built-in `gen_random_uuid()`. PostGIS: Docker uses `postgis/postgis`; `make migrate` enables it on `lowcode_data`; self-hosted/cloud: [data/README.md](../migrations/data/README.md).

SQL files are idempotent (`IF NOT EXISTS`). Re-run `make migrate` after adding `NNNN_xxx.up.sql`.

**Do not** embed SQL schema in `internal/service` or `cmd/server`.

## API overview

Prefix `/v1/`, JSON camelCase, **`X-Tenant-Id`** required.

| Domain | Main routes |
|--------|-------------|
| Tenant | `POST /v1/admin/tenants` (`recordStore`: shared \| dedicated; DB isolation via `dataDsn`) |
| Table | `GET/POST /v1/admin/tables`, `GET .../schema`, `POST ...:rename`, `DELETE .../{name}` |
| Column | `GET/POST /v1/admin/columns?table_id=`, `PATCH/DELETE /v1/admin/columns/{id}` |
| Row | `GET/POST .../rows`, `POST .../rows:query`, `PATCH/DELETE .../rows/{id}`, bulk/import |
| Index | `GET/POST /v1/admin/indexes?table_id=`, `GET/DELETE /v1/admin/indexes/{pgIndexName}` |
| Relation | `GET/POST /v1/admin/relations`, `DELETE /v1/admin/relations/{id}` |
| Query | `GET/POST /v1/admin/queries`, `POST /v1/data/queries/{name}` (define + execute) |
| ER | `GET /v1/admin/schema/er` |

## Domain conventions

### Index

- **Source of truth:** PostgreSQL catalog
- Create: `CREATE [UNIQUE] INDEX IF NOT EXISTS idx_{table}_{name} ON ...`
- List/schema: `listPGIndexes` + `pgIndexesToAPI`
- `Index.Id` = PG index name (e.g. `idx_vendor_score`)

### Virtual columns

| kind | Notes |
|------|-------|
| link | one-to-many / many-to-one; `link_column_id` and `target_column_id` are mutually exclusive in config |
| lookup | LEFT JOIN via cardinality-one link |
| formula | Excel expression (`config.expression`); `{{column_name}}` → [efp](https://github.com/xuri/efp) AST, evaluated in the calc worker |
| rollup | Aggregate subquery on the related table |

### Query

- DSL: `internal/dsl` (metadata-compatible JSON shape)
- Merge: saved Query filter + sort + columnIds; `POST .../rows:query` or `POST /v1/data/queries/{name}`

## When changing code

1. **API types:** `internal/apiv1/<domain>/`
2. **Business:** `internal/service/<module>/` (schema · catalog · data · platform) — [docs/modules/README.md](../docs/modules/README.md)
3. **Routes:** `internal/api/routes.go` (chi); handlers in `admin/`, `data/`
4. **Tests:** unit tests in-package; integration `internal/service/integration_test.go` + `internal/testutil`

Keep diffs small; match existing domain subpackage layout.

Out of scope: [docs/roadmap.md](../docs/roadmap.md) (schema bundle import, plugins, graph query, Choice/ENUM, RBAC).

## Pitfalls

- Missing `X-Tenant-Id` → 400
- `loadColumns` excludes virtual columns; index lookup uses `loadTablePhysical`
- README mentions of proto / TENANT_MODE / gRPC are stale; this file and `AGENTS.md` win
