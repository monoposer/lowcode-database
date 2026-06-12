# Lowcode Database (Postgres + HTTP JSON)

**[English](README.md) | [中文](README.zh.md)**

A Postgres-backed low-code table service with dynamic schema, row CRUD, virtual columns, data-source queries, and event delivery.

## Features

- **Table / Column / Index / Choice (PG ENUM)** — schema managed via Admin API; DDL applied to tenant data DB
- **Row CRUD** — create, update, delete, bulk upsert/delete, import, nested `saveGraph`
- **Virtual columns** — `relationship`, `lookup`, `formula`, `rollup` (no physical PG column)
- **relation_fk** — physical FK column with optional `FOREIGN KEY` constraint
- **DataSource** — column projection + filter + sort; query via Data API
- **Relations & ER diagram** — `lc_relations` + schema ER API
- **Relationship expand** — `expand_column_ids` / `expand_paths` on list rows
- **Event delivery** — async push to configured sinks (HTTP, Redis, Kafka, RabbitMQ, …); see [pkg/eventschema](pkg/eventschema/)
- **HTTP JSON API** — `net/http`, no gRPC; split into **Admin** and **Data** prefixes
- **Dual database** — shared meta DB + per-tenant data DB

## Requirements

- Go 1.22+
- Postgres (local: `make docker-up` starts Postgres + Redis)

## Quick start

```bash
cp .env.example .env
make docker-up    # postgres + redis
make migrate      # apply SQL migrations
make run          # http://localhost:8080
```

Every API request needs header `X-Tenant-Id: default` (or your tenant id).

After startup:

- **HTTP API**: `http://localhost:8080/`
- **OpenAPI + Swagger UI**: `http://localhost:8080/swagger/`
- **Version**: `GET /` or `./server -version`

### Configuration

| Variable | Description |
|----------|-------------|
| `META_DATABASE_URL` | Meta DB (`lc_*` tables, tenants) |
| `DEFAULT_TENANT_DATA_DSN` | Default tenant data DB (`lc_t_*` physical tables) |
| `DEFAULT_TENANT_ID` | Bootstrap tenant id (default `default`) |
| `REDIS_URL` + `CACHE_ENABLED` | Optional metadata cache |
| `API_KEY_REQUIRED` | Require `X-Api-Key` on `/v1/*` |
| `AUTHZ_DRIVER` | `file` or `http` for RBAC (see `config/authz.example.json`) |

See [`.env.example`](.env.example) for full options.

### Multi-tenant

Each tenant has its own data DSN stored in meta (`lc_tenants`). Create tenants via `POST /v1/admin/tenants`. Pass `X-Tenant-Id` on every request.

## API layout

Legacy `/v1/tables`, `/v1/webhooks`, etc. return **404**. Use:

| Prefix | Purpose | Examples |
|--------|---------|----------|
| `/v1/admin/*` | Schema & platform | `GET /v1/admin/tables`, `POST /v1/admin/columns`, `GET /v1/admin/schema/er` |
| `/v1/data/*` | Row reads/writes & queries | `GET /v1/data/tables/{tableId}/rows`, `POST /v1/data/tables/{tableId}/rows:saveGraph` |

OpenAPI spec: [`internal/api/openapi/openapi.yaml`](internal/api/openapi/openapi.yaml)

## Event delivery

Configure **event sinks** under Admin API. After row or schema changes, the service asynchronously pushes JSON envelopes to the sink delivery URL.

Envelope shape (see [pkg/eventschema](pkg/eventschema/README.md)):

```json
{
  "type": "records.after.insert",
  "tenantId": "default",
  "tableId": "orders",
  "occurredAt": "2026-06-12T10:00:00Z",
  "data": { "row": { "id": "...", "amount": { "numberValue": 99.5 } } }
}
```

**Admin endpoints:**

- `GET /v1/admin/event-sinks` — list
- `POST /v1/admin/event-sinks` — create (`name`, `deliveryUrl`, `events`, `tableFilter`, `enabled`, …)
- `PATCH /v1/admin/event-sinks/{id}` — update
- `DELETE /v1/admin/event-sinks/{id}` — delete
- `GET /v1/admin/events/schemas` — JSON Schemas for all event types

**Delivery:** HTTP POST to an `http://` or `https://` `targetUrl` only. For Kafka, Redis, RabbitMQ, SQS, or SNS, deploy an **HTTP adapter** and point `targetUrl` at it. Auth: custom headers plus optional HMAC via `secret` (`X-Lowcode-Signature`). See [docs/event-delivery.md](docs/event-delivery.md).

**Subscription rules:**

- `events` empty → all row-level `records.*` types
- `events` non-empty → only listed types
- `tableFilter` non-empty → only matching logical table name

## Playground (debug UI)

Separate repo: [lowcode-database-playground](https://github.com/solat/lowcode-database-playground)

```bash
make run   # API on :8080

git clone https://github.com/solat/lowcode-database-playground.git
cd lowcode-database-playground
cp .env.example .env && npm install && npm run dev   # :5173
```

Set **X-Tenant-Id** in the sidebar (default `default`).

## Relationship, lookup & expand

### relationship (virtual)

No PG storage column. In `config`, set **`link_column_id` OR `target_column_id`** (not both):

- **many** — child table has FK column pointing to current row (`link_column_id`)
- **one** — current table has FK column pointing to target row (`target_column_id`)

**ListRows — `expand_column_ids`:**

- `many` → `{ "rows": [ { "id", "cells": { … } }, … ] }`
- `one` → `{ "id", "cells": { … } }` or `null`

Example: `GET /v1/data/tables/{tableId}/rows?expand_column_ids=col-uuid-1`

### expand_paths

Comma-separated dot paths (column ids), min 2 segments, max depth 5. Example:

`GET /v1/data/tables/{tableId}/rows?expand_paths=relToOrder.orderName`

Result key is the full path string.

### lookup (virtual)

Projects a physical column from a **one** relationship. Config: `relation_column_id`, `target_column_id`. Computed in list/query via JOIN; read-only on single-row write API.

### relation_fk (physical)

Real PG column matching target type. Config: `target_table_id`, optional `target_column_id`, `add_fk`.

### formula / rollup

Read-only computed columns (`expression` / aggregation over relationship).

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
| Scalar / choice / relation_fk | yes | Direct PG column |
| relationship | no | Use link FK or target FK column |
| lookup | no (single-row API) | Write underlying FK; or use `saveGraph` |
| formula / rollup | no | Read-only |

### Common Data API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/data/tables/{tableId}/rows` | Paginated list + expand |
| POST | `/v1/data/tables/{tableId}/rows` | Create row |
| PATCH | `/v1/data/tables/{tableId}/rows/{rowId}` | Update row |
| DELETE | `/v1/data/tables/{tableId}/rows/{rowId}` | Delete row |
| POST | `/v1/data/tables/{tableId}/rows:query` | DSL filter query |
| POST | `/v1/data/tables/{tableId}/rows:bulkUpsert` | Bulk upsert (single table, transactional) |
| POST | `/v1/data/tables/{tableId}/rows:saveGraph` | Nested save (main row + relationships) |
| POST | `/v1/data/tables/{tableId}/rows:bulkDelete` | Bulk delete by ids |

## Nested save `rows:saveGraph`

`POST /v1/data/tables/{tableId}/rows:saveGraph` saves the main row and nested relationship data in one transaction:

| cardinality | payload | behavior |
|-------------|---------|----------|
| **one** | object | create/update related row, set FK on main row |
| **many** | array | upsert child rows, fill link column |

Use `"_sync": { "items": "replace" }` to replace all child rows for a many relationship.

## Commands

```bash
make migrate          # apply meta + data SQL
make run              # start HTTP server
make test             # unit tests (integration needs TEST_META_DATABASE_URL)
make docker-build     # build image (see deploy/)
make docker-up        # postgres + redis only
make docker-up-stack  # full stack including app (deploy/docker-compose.yml)
```

## Deploy & release

- Docker: [`deploy/Dockerfile`](deploy/Dockerfile), [`deploy/docker-compose.yml`](deploy/docker-compose.yml)
- Release workflow: tag `v*.*.*` → Docker Hub ([`deploy/RELEASE.md`](deploy/RELEASE.md))
- Version file: [`VERSION`](VERSION)

## Project layout

| Path | Description |
|------|-------------|
| `cmd/server/` | HTTP entry |
| `cmd/migrate/` | Schema migration CLI |
| `internal/api/` | Routes & handlers |
| `internal/apiv1/` | Hand-written JSON types |
| `internal/service/` | Business logic |
| `internal/event/` | Event bus & delivery |
| `pkg/eventschema/` | Public event JSON Schemas |
| `docker/postgres/migrations/` | SQL migrations |

## Learn more

- [docs/](docs/README.md) — architecture (中文), module index, design analysis
- [AGENTS.md](AGENTS.md) — agent/developer quick reference
