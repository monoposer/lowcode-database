# Internal modules

Package index for `lowcode-database`. See [技术架构.md](技术架构.md) for the full architecture (Chinese).

## `cmd/`

| Entry | Description |
|-------|-------------|
| `cmd/server` | Load config → TenantManager → event Bus → LowcodeService → authn/authz middleware → HTTP |
| `cmd/migrate` | Apply `docker/postgres/migrations/` to meta and/or data targets |

## `internal/api`

HTTP routing and JSON handlers for `/v1/admin/*` and `/v1/data/*` ([chi](https://github.com/go-chi/chi) router).

| Path | Description |
|------|-------------|
| `routes.go` | Single route table — all endpoints registered here |
| `httputil/base.go` | JSON read/write, tenant + writable middleware |
| `admin/platform.go` | Tenant, connection, API keys, events, ER diagram |
| `admin/schema.go` | Tables, columns, indexes, choices, relations, data sources |
| `data/row.go` | Row CRUD, query, saveGraph, bulk, import/export |
| `openapi/` | Static OpenAPI 3 spec + Swagger UI |

Middleware chain (in `cmd/server`): CORS → request log → `authn.Validator` → `authz.Middleware` → api handler.

## `internal/apiv1`

Hand-written JSON request/response types (no protobuf). Domain subpackages — see **`doc.go`**.

| Subpackage | Description |
|------------|-------------|
| *(root)* | `Value`, `SortOrder`, cell parse/convert helpers |
| `schema/` | Table, Column, Index, Choice, Relation, ER types |
| `row/` | Row entity, CRUD/import/export, flat JSON marshal |
| `datasource/` | DataSource admin + query types |
| `graph/` | SaveGraph nested relationship save |
| `platform/` | Tenant, API keys, event sinks, observability |

## `internal/service`

Business logic facade: `LowcodeService` embeds domain services sharing `shared.Base`.

| Subpackage | Description |
|------------|-------------|
| `schema` | Table/column DDL — `table.go`, `column_mutate.go`, `column_virtual.go`, `virtual.go` (see `doc.go`) |
| `data` | Row CRUD, query, saveGraph, bulk — `row.go`, `query_exec.go`, `lookup.go`, `savegraph.go` (see `doc.go`) |
| `catalog` | PG ENUM + index — `pg_enum.go`, `choice_*.go`, `pg_index.go` |
| `graph` | `lc_relations` CRUD |
| `platform` | Tenants, API keys, data sources, event sinks, observability list APIs |
| `meta` | Cross-domain metadata read facade (columns, relationships, indexes, choices) |
| `shared` | Base, cells, config, `result_type.go`, `helpers.go` |

## `internal/dsl` + `internal/query`

Filter DSL parsing (`dsl.go`, `params.go`) and SQL builder (`query/builder.go` — SELECT/ORDER, `query/where.go` — WHERE) for data queries.

## `internal/formula` + `internal/columntype`

Formula expression compile/plan and built-in column type registry.

## `internal/event`

Async event bus: publishes JSON envelopes to HTTP endpoints configured in `lc_event_sinks` (renamed from `lc_webhooks`; same as webhook subscriptions). Broker delivery (Rabbit, Kafka, SQS, SNS, Redis) uses external HTTP adapters — see [event-delivery.md](event-delivery.md).

| Path | Description |
|------|-------------|
| `bus.go` | Fan-out + delivery routing |
| `delivery/http.go` | POST + optional HMAC signature |
| `schemas.go` | Expose `pkg/eventschema` + metrics via Admin API |

See [internal/event/README.md](../internal/event/README.md).

## `internal/infra/postgres`

`TenantManager`: meta pool + per-tenant data pools — `tenant_manager.go`, `tenant_pool.go`, `pg.go` (see `doc.go`).

## `internal/infra/redis`

Optional Redis client for cache and redis-backed metrics.

## `internal/platform/cache`

Redis-backed metadata cache for data sources, views, column specs; invalidated on writes.

## `internal/platform/metrics`

Rolling average query latency per data source — `metrics.go` + `backends.go` (noop / redis / prometheus).

## `internal/platform/authn`

API key authentication against `lc_api_keys` when `API_KEY_REQUIRED=true`.

## `internal/platform/authz`

RBAC middleware: `file` (JSON rules) or `http` (delegate to external authorize URL). Route mapping in `routes.go`; file driver in `file.go`; HTTP driver in `authz.go`.

## `internal/config`

Environment variable loading from `.env` and process env.

## `internal/logger`

JSON structured logging; optional SQL and slow-query logging.

## `internal/migrator`

SQL migration runner used by `cmd/migrate`.

## `internal/tenant`

Request-scoped tenant ID in context.

## `internal/telemetry`

Tracing hook interface (default noop).

## `pkg/eventschema`

Public JSON Schemas for event envelope and per-type payloads; embedded for Admin `GET /v1/admin/events/schemas`.

## `docker/postgres/migrations/`

| 目录 | 说明 |
|------|------|
| `meta/` | Versioned SQL（`000001_init.up.sql` 等） |
| `data/` | **无 `.up.sql`** — 物理表运行时 DDL；PG 16+ 要求与可选 PostGIS 见 [data/README.md](../docker/postgres/migrations/data/README.md) |

## `deploy/`

Dockerfile, compose stack, release notes — see [deploy/README.md](../deploy/README.md).
