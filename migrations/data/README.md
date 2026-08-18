# Tenant data DB

Physical tables, indexes, and runtime DDL are created by the service (`EnsureDataTables` in `pkg/infra/postgres`). There is **no** `cmd/migrate` target for data databases.

`EnsureDataTables` also runs `CREATE EXTENSION IF NOT EXISTS` for **pg_stat_statements**. The Postgres server still needs `shared_preload_libraries=pg_stat_statements` when you enable SQL stats (compose sets this).

## PostgreSQL version

Requires **PostgreSQL 16+**.

## Extensions

| Extension | Notes |
|-----------|-------|
| **pgcrypto** | Not needed. `gen_random_uuid()` is built-in since PG 13. |
| **pg_stat_statements** | SQL stats when `PG_STAT_STATEMENTS=true`. Needs `shared_preload_libraries=pg_stat_statements` (compose already sets this). |

## Local Docker

`deploy/docker-compose.yml` uses **`postgres:16`**.

`lowcode_data` is only the default tenant DB. When Admin creates a tenant with a **dedicated database**, the same extensions run via `EnsureDataTables` on that DSN.
