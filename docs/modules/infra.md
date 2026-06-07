# infra module

Database connections, tenant context, migrations, config.

## internal/infra/postgres

**TenantManager** — dual-DB core.

| File | Role |
|------|------|
| `tenant_manager.go` | Meta pool, default tenant bootstrap |
| `scope.go` | `tenant_id` / `base_id` WHERE helpers |
| `tenant_pool.go` | Lazy Data pools by tenant id, CreateTenant |
| `tenant_schema.go` | shared_db schema, reserved-name checks |
| `rls_shared.go` | rls_table: `lc_shared` + RLS session |
| `pg.go` | pgxpool creation |

### Pool API

- `MetaPool()` — global meta
- `DataPool(ctx)` — tenant write DSN (`data_dsn` / `data_dsn_write`)
- `DataReadPool(ctx)` — replicas (`data_dsn_reads`, round-robin); `X-Read-Consistency: strong` uses primary

See [system.md](../architecture/system.md) (connection pools) and [tenant-isolation.md](../architecture/tenant-isolation.md).

## internal/infra/redis

Optional Redis; used by cache / redis metrics.

## internal/tenant

`X-Tenant-Id` → `context.Context` (written by middleware).

## internal/migrator

`cmd/migrate` reads embedded `migrations/` (or `-dir`).

## internal/config

`.env` + environment: `META_DATABASE_URL`, `VR_DEFAULT_SHARD_DSN`, …

## Dependency direction

`infra` must not import `service` / `api`.

## Meta migrations

| File | Contents |
|------|----------|
| `000001_init.up.sql` | Full meta: tenants (incl. replica DSNs + `record_store`), tables, columns, indexes, columnTypes, webhooks |

Data DB `000001_postgis.up.sql` enables PostGIS; business tables are created by **schema** runtime DDL.
