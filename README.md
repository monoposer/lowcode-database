# Lowcode Database (Postgres + HTTP JSON)

**[English](README.md) | [中文](README.zh.md)**

A Postgres-backed low-code table service: dynamic schema, row CRUD, virtual columns, and saved queries.

Isolation unit is the **tenant** (`tenants.tenant_id`): header `X-Tenant-Id`. Each tenant has one **`data_dsn`** used for both reads and writes.

## Features

- **Table / Column / Index** — schema via Admin API; DDL on the tenant data DB
- **Row CRUD** — create, update, delete, bulk upsert/delete, import
- **Link** — virtual column (`type_id=link`); IDs live in `link_ref`, not `record.data`
- **lookup / formula / rollup** — cached in `record.data` via `calc_queue` + in-process worker
- **Query** — saved projection + filter + sort; execute with `POST /v1/data/queries/{name}`
- **ER diagram** — edges derived from Link columns (`GET /v1/admin/schema/er`)
- **HTTP JSON** — `net/http`, no gRPC; admin + data + calc in **one process**
- **Dual database** — shared meta DB + per-tenant data DSN

## Requirements

- Go 1.25+
- Postgres (local: `make docker-up` starts Postgres + Redis)

## Quick start

```bash
cp .env.example .env
make docker-up         # postgres + redis (empty DBs; does not apply SQL)
make migrate           # apply SQL migrations
make run               # :8080  /v1/admin + /v1/data + calc
make playground-dev    # :5173  debug UI (web/playground)
```

Every API request needs `X-Tenant-Id`.

After startup:

- **Control (Admin)**: `http://localhost:8080/v1/admin/*`
- **Data**: `http://localhost:8080/v1/data/*`
- **OpenAPI + Swagger UI**: `http://localhost:8080/swagger/`
- **Playground**: `http://localhost:5173` (`make playground-dev`)

### Configuration

| Variable | Description |
|----------|-------------|
| `META_DATABASE_URL` | Meta DB (`tenants`, `lc_*`) |
| `DATA_DATABASE_URL` | Local default tenant data DSN (pass as `data_dsn` when creating a tenant) |
| `HTTP_ADDR` | Listen address (default `:8080`) |
| `REDIS_URL` + `CACHE_ENABLED` | Optional metadata cache |
| `API_KEY_REQUIRED` | Require `X-Api-Key` on `/v1/*` |
| `PG_STAT_STATEMENTS` | `true` enables `GET /v1/admin/pg-stat-statements` |

See [`.env.example`](.env.example) for full options.

### Multi-tenant

Each tenant is a `tenants` row with **`data_dsn`**. Create via `POST /v1/admin/tenants` (also seeds a **public** base and a default API key; plaintext `key` is returned once), then send `X-Tenant-Id` on every request. `cmd/migrate` only applies **meta** schema and does not bootstrap tenants.

## API layout

| Prefix | Process | Purpose |
|--------|---------|---------|
| `/v1/admin/*` | `cmd/server` | Schema & platform (Meta) |
| `/v1/data/*` | `cmd/server` | Row reads/writes, execute saved queries |

OpenAPI: [`internal/api/openapi/openapi.yaml`](internal/api/openapi/openapi.yaml)

Optional API Key auth: `API_KEY_REQUIRED` + `X-Api-Key`. There is no RBAC in this service.

## Playground (debug UI)

In-repo: [`web/playground`](web/playground).

```bash
make run              # Admin + Data + calc :8080
make playground-dev   # UI :5173
```

Set **X-Tenant-Id** in the sidebar (default `default`).

## Link & lookup

Link metadata is in `lc_columns` (`type_id=link`). **Related IDs exist only in `link_ref`**, not in `record.data`.

Lookup / formula / rollup results are cached in `record.data` (`{value, _cache_status}`), refreshed by the `calc_queue` worker. List reads the cache; detail `GET /v1/data/tables/{id}/rows/{rowId}` can compute live. See [docs/architecture/record-calc.md](docs/architecture/record-calc.md).

### link (virtual)

No standalone PG column. `config` needs `target_table_name` (or `to_table_name`). Optional:

- **many** — `cardinality=many` and/or `link_column_id` on the related table
- **one** — `cardinality=one` and/or `target_column_id`

Row APIs write Link as IDs (stored in `link_ref`). Graph expand / `saveGraph` is [roadmap](docs/roadmap.md#graph-query).

### lookup / formula / rollup

Cached in `record.data`; not writable on row APIs. Worker refreshes via `calc_queue`.

## Row JSON format

Create, update, and list responses use **flat rows**: column names alongside `id`, native JSON scalars.

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "amount": 99.5,
  "vendor_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

| Column kind | Writable | Notes |
|-------------|----------|-------|
| Scalar / choice | yes | Stored in `record.data` |
| link | yes (IDs) | Persisted in `link_ref` |
| lookup | no | Write the underlying link IDs |
| formula / rollup | no | Read-only cache |

### Common Data API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/data/tables/{tableName}/rows` | Paginated list |
| GET | `/v1/data/tables/{tableName}/rows/{rowId}` | Detail (live formula/lookup/rollup) |
| POST | `/v1/data/tables/{tableName}/rows` | Create row |
| PATCH | `/v1/data/tables/{tableName}/rows/{rowId}` | Update row |
| DELETE | `/v1/data/tables/{tableName}/rows/{rowId}` | Delete row |
| POST | `/v1/data/tables/{tableName}/rows:query` | DSL filter query |
| POST | `/v1/data/tables/{tableName}/rows:bulkUpsert` | Bulk upsert (single table, transactional) |
| POST | `/v1/data/tables/{tableName}/rows:bulkDelete` | Bulk delete by ids |
| POST | `/v1/data/queries/{name}` | Execute a saved query |

## Commands

```bash
make migrate          # apply meta + data SQL
make run              # admin + data + calc
make playground-dev   # debug UI
make test             # unit tests (integration needs TEST_META_DATABASE_URL)
make docker-build     # build image (see deploy/)
make docker-up        # postgres + redis only (empty DBs; no auto SQL)
make docker-migrate   # apply SQL via compose migrate service
make docker-up-stack  # full stack including app (deploy/docker-compose.yml)
```

## Deploy & release

- Docker: [`deploy/Dockerfile`](deploy/Dockerfile), [`deploy/docker-compose.yml`](deploy/docker-compose.yml)
- Release workflow: tag `v*.*.*` → Docker Hub ([`deploy/RELEASE.md`](deploy/RELEASE.md))
- Version file: [`VERSION`](VERSION)

## Project layout

| Path | Description |
|------|-------------|
| `cmd/server/` | Runtime: `/v1/admin/*` + `/v1/data/*` + calc |
| `cmd/migrate/` | Schema migration CLI |
| `web/playground/` | Debug UI |
| `internal/api/` | Routes & handlers |
| `internal/service/{schema,catalog,data,platform,shared}/` | Hand-written JSON types (no proto) |
| `internal/service/` | Business logic |
| `migrations/` | SQL migrations |

## Learn more

- [docs/](docs/README.md) — architecture, module index
- [AGENTS.md](AGENTS.md) — agent/developer quick reference
