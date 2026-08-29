# Local development (Cursor)

## Environment

# Dual-schema model on one Postgres database (`lowcode`):

```bash
# Meta: all lc_* metadata + tenants (schema `meta`)
META_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/lowcode?search_path=meta

# Migrate applies meta only. This DSN is what you pass as tenant data_dsn (row tables in public).
DATA_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/lowcode

HTTP_ADDR=:8080
MAX_ROW=100

# Optional: Redis metadata cache
# REDIS_URL=redis://localhost:6379/0
# CACHE_ENABLED=true
# CACHE_TTL_SECONDS=300
# PG_STAT_STATEMENTS=true      # GET /v1/admin/pg-stat-statements
LOG_LEVEL=debug
# SLOW_QUERY_THRESHOLD_MS=500

# Optional: CREATE DATABASE when Admin creates a tenant
# DATA_ADMIN_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/postgres
# DATA_DSN_TEMPLATE=postgresql://postgres:postgres@localhost:5432/%s
```

Copy: `cp .env.example .env` and edit as above.

## Run and debug

```bash
make docker-up      # postgres:16; schema meta on database `lowcode`; rows in public
make migrate        # or make docker-migrate / go run ./cmd/migrate
make run            # HTTP service (does not migrate)
make test
make test-integration
```

Playground UI: in-repo `web/playground` (`make playground-dev`).

Integration tests:

```bash
export TEST_META_DATABASE_URL='postgresql://postgres:postgres@localhost:5432/lowcode?search_path=meta'
export TEST_DATA_DATABASE_URL='postgresql://postgres:postgres@localhost:5432/lowcode'
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

SQL: `migrations/meta/` (`cmd/migrate`). Data tables/extensions: runtime DDL (`EnsureDataTables`); notes: [data/README.md](../migrations/data/README.md).

| Command | Notes |
|---------|-------|
| `make docker-up` | postgres + redis only; empty volume init only CREATE DATABASE |
| `make migrate` | `cmd/migrate` apply meta (embed `migrations/meta`) |
| `make docker-migrate` | compose `run --rm migrate` (not started by docker-up) |
| `go run ./cmd/migrate -database-url '...'` | Meta migrate against an explicit URL |

**Data DB:** business tables and extensions (optional `pg_stat_statements`) use runtime DDL. UUID PKs use built-in `gen_random_uuid()`. Docker uses `postgres:16`; self-hosted/cloud: [data/README.md](../migrations/data/README.md).

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

1. **API types:** next to the domain in `internal/service/<module>/` (`shared.Value` for cells)
2. **Business:** `internal/service/<module>/` (schema · catalog · data · platform) — [docs/modules/README.md](../docs/modules/README.md)
3. **Routes:** `internal/api/routes.go` (chi); handlers in `admin/`, `data/`
4. **Tests:** unit tests in-package; integration `internal/service/integration_test.go` + `internal/testutil`

Keep diffs small; match existing domain subpackage layout.

Out of scope: [docs/roadmap.md](../docs/roadmap.md) (schema bundle import, plugins, graph query, Choice/ENUM, RBAC).

## Pitfalls

- Missing `X-Tenant-Id` → 400
- `loadColumns` excludes virtual columns; index lookup uses `loadTablePhysical`
- README mentions of proto / TENANT_MODE / gRPC are stale; this file and `AGENTS.md` win
