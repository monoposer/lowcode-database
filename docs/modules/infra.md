# infra module

Database connections, tenant context, migrations, config.

## pkg/infra/postgres

**TenantManager** — dual-DB core.

| File | Role |
|------|------|
| `tenant_manager.go` | Meta pool (no startup tenant bootstrap) |
| `record_store.go` | `record_store=shared` vs `{tenant_id}_record` |
| `tenant_schema.go` | Ensure data tables on tenant DSN |
| `pg.go` | pgxpool creation |

### Pool API

- `MetaPool()` — global meta
- `DataPool(ctx)` — tenant write DSN (`data_dsn` / `data_dsn_write`)
- `DataReadPool(ctx)` — replicas (`data_dsn_reads`, round-robin); `X-Read-Consistency: strong` uses primary

See [system.md](../architecture/system.md) (connection pools) and [tenant-isolation.md](../architecture/tenant-isolation.md).

## pkg/infra/redis

Optional Redis; used by cache / redis metrics.

## pkg/tenant

`X-Tenant-Id` → `context.Context` (written by middleware).

## pkg/migrator

`cmd/migrate` reads embedded `migrations/meta/` (or `-dir`).

## pkg/config

`.env` + environment: `META_DATABASE_URL`, `DATA_DSN_TEMPLATE`, …

## Dependency direction

`infra` must not import `service` / `api`.

## Meta migrations

| File | Contents |
|------|----------|
| `000001_init.up.sql` | Full meta: tenants (incl. replica DSNs + `record_store`), tables, columns, indexes, columnTypes, webhooks |

Data tables are created by **schema** runtime DDL (`EnsureDataTables`).
