# lowcode-database — Cursor / Agent notes

Postgres-backed low-code table service: HTTP JSON API (`/v1/*`), dynamic tables/columns/rows/views, dual-DB architecture (meta + data).

## Local startup

1. `cp .env.example .env`
2. **Docker (recommended)**: `make docker-up` — Postgres + Redis (empty DBs; **does not** apply SQL)
3. `make migrate` (`cmd/migrate`) or `make docker-migrate` — apply schema
4. `make run` → http://localhost:8080 (`/v1/admin/*`, `/v1/data/*`, calc)
5. `make playground-dev` → http://localhost:5173
6. APIs require `X-Tenant-Id`

## Layout

| Path | Notes |
|------|-------|
| `cmd/server/` | Runtime: `/v1/admin/*` + `/v1/data/*` + in-process calc worker; mounts `/playground/` when `web/playground/dist` exists |
| `cmd/migrate/` | Schema migration CLI |
| `web/playground/` | Vite + AG Grid debug UI (hash pages `#/editor`, etc.) |
| `internal/api/` | Routes and handlers (`/v1/admin/*`, `/v1/data/*`) |
| `internal/service/` | Domain modules + JSON types: `schema`, `catalog`, `data`, `platform`, `calc`, `shared` (see [docs/modules/](docs/modules/README.md)) |
| `internal/columntype/` | Built-in pgTypes and tenant columnType specs |
| `internal/service/shared/` | Cross-domain helpers (result type, cells, config) |
| `internal/dsl/`, `internal/query/` | Filter DSL → SQL |
| `internal/event/` | Event types + EventBus (memory / Redis Stream) + webhooks |
| `pkg/infra/postgres/` | Dual-DB TenantManager, pools |
| `pkg/infra/redis/` | Redis client (optional) |
| `pkg/platform/cache/` | Redis metadata cache (query / column spec) |
| `pkg/platform/metrics/` | `pg_stat_statements` list (`PG_STAT_STATEMENTS`) |
| `pkg/platform/authn/` | API Key validation (authentication) |
| `pkg/logger/` | JSON structured logs |
| `pkg/config/` | Env / `.env` |
| `pkg/tenant/` | `X-Tenant-Id` on context |
| `pkg/migrator/` | Schema migration runner (`cmd/migrate`) |
| `migrations/` | Meta/Data SQL (embedded by `cmd/migrate`) |
| `docs/` | Architecture docs ([docs/README.md](docs/README.md)) |

Debug UI: in-repo **`web/playground`** (`make playground-dev`).

## Architecture

- **Meta DB**: `tenants` (`data_dsn`), `lc_bases`, `lc_tables`, `lc_columns`, `lc_queries`, `lc_indexes`
- **Data DB**: `record` (LIST partition by `vt_id`) or `{tenant_id}_record` when `tenants.record_store=dedicated`; also `link_ref`, `calc_queue`
- **Virtual columns** (`formula` / `link` / `lookup` / `rollup`) have no physical column; results cache in `record.data`; relations in `link_ref`
- **Index**: meta `lc_indexes` + PG catalog DDL; `POST /indexes:backfill` backfills

## Performance and observability

- **Redis cache** (`REDIS_URL` + `CACHE_ENABLED`): caches query / column metadata; writes invalidate
- **SQL stats**: `PG_STAT_STATEMENTS=true` enables `GET /v1/admin/pg-stat-statements` (Postgres records automatically)
- **Logs**: JSON stdout; `SLOW_QUERY_THRESHOLD_MS` warns on slow query / SQL

```bash
make docker-up      # postgres + redis (empty DBs)
make migrate        # apply migrations/
export REDIS_URL=redis://localhost:6379/0
export PG_STAT_STATEMENTS=true
make run
```

## Do not confuse

- No gRPC / protobuf; do not add `make proto`
- Access layer is pgx only (no Ent); static SQL includes `tenant_id`/`base_id`
- `Table.Id` is the logical **name**, not a UUID
- Virtual columns (`formula` / `link` / `lookup` / `rollup`) have no physical column; cache in `record.data`
- Schema changes: **edit `migrations/`**, run `make migrate` (or `make docker-migrate`)
- Business services **do not** auto-migrate
- Out of scope this version: [docs/roadmap.md](docs/roadmap.md) (schema bundle import, plugins, graph query, Choice/ENUM, RBAC)

See [.cursor/DEVELOPMENT.md](.cursor/DEVELOPMENT.md) · architecture [docs/README.md](docs/README.md) ([system](docs/architecture/system.md), [analysis](docs/architecture/analysis.md), [record calc](docs/architecture/record-calc.md), [Virtual-Records RFC](docs/architecture/virtual-records.md))
