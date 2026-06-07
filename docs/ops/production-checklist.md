# Production deploy checklist (Phase-1, single process)

Tick before going live. Domain model is unchanged. **RBAC is not in this service** (external authz).

## Process topology

- [ ] **One runtime**: `cmd/server` on `HTTP_ADDR` (`:8080`) serves `/v1/admin/*`, `/v1/data/*`, `/v1/worker/*`, and polls `calc_queue`
- [ ] `cmd/migrate` is CLI only; the server does **not** auto-migrate
- [ ] Horizontal scale = N identical server replicas (set `EVENT_BUS=redis`; calc uses `SKIP LOCKED`)
- [ ] New data DSN: `make migrate` (or `make docker-migrate`) so PostGIS / `pg_stat_statements` land on that database

## Read/write split

- [ ] Tenant `data_dsn` / `data_dsn_write` = primary; `data_dsn_reads` = replica list (`PATCH /v1/admin/tenants/{id}`)
- [ ] Row query / export / saved-query use replicas; writes use primary
- [ ] Lag-sensitive reads: `X-Read-Consistency: strong` or JSON `consistency: "strong"`

## Event bus

- [ ] Single instance: `EVENT_BUS=memory` is enough
- [ ] Multiple replicas: `EVENT_BUS=redis` + `REDIS_URL`
- [ ] External consumers: `/v1/admin/webhooks` (do not poll `lc_schema_audit`)

## Database

- [ ] `META_DATABASE_URL` separate from per-tenant `data_dsn`
- [ ] Shared data DSN: reuse pgxpool **by DSN**
- [ ] `MAX_TENANT_DATA_POOLS` vs Postgres `max_connections` (count write + replica DSNs)
- [ ] Watch `GET /v1/admin/runtime`

## Data DB extensions

- [ ] `make migrate` applies `migrations/data` (PostGIS, `pg_stat_statements`) to each unique tenant write DSN

## Overload / calc / cache / security

- [ ] Rate limits, `MAX_ROW` / `MAX_SCAN_ROWS` / `MAX_BULK_ITEMS` / `MAX_EXPORT_ROWS`
- [ ] `DDL_CONFIRM_REQUIRED=true`
- [ ] `CALC_WORKER_BATCH`, `CALC_TENANT_CONCURRENCY`, `CALC_MAX_RETRY`
- [ ] Redis cache TTL; `API_KEY_REQUIRED=true` in production
- [ ] `GET /health` for liveness + version
