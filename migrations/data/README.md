# Tenant data DB

Physical tables, indexes, and runtime DDL are created by the service (`EnsureDataTables` in `pkg/infra/postgres`). There is **no** `cmd/migrate` target for data databases.

`EnsureDataTables` also runs `CREATE EXTENSION IF NOT EXISTS` for **postgis** and **pg_stat_statements**. The Postgres server still needs the binaries / `shared_preload_libraries` (compose sets both).

## PostgreSQL version

Requires **PostgreSQL 16+**.

## Extensions

| Extension | Notes |
|-----------|-------|
| **pgcrypto** | Not needed. `gen_random_uuid()` is built-in since PG 13. |
| **postgis** | Required for `geometry` / `geography` / `point` columns. See below. |
| **pg_stat_statements** | SQL stats when `PG_STAT_STATEMENTS=true`. Needs `shared_preload_libraries=pg_stat_statements` (compose already sets this). |

---

## Enabling PostGIS

PostGIS is **not** part of core PostgreSQL. Two steps:

1. **Install PostGIS software** (binaries and SQL scripts for the `postgis` extension)
2. Open a tenant data database (extensions are created on first `EnsureDataTables`)

Missing either step, creating a `geometry` column fails with `type "geometry" does not exist` or `extension "postgis" is not available`.

### Local Docker (recommended)

`deploy/docker-compose.yml` uses **`postgis/postgis:16-3.5`** with `platform: linux/amd64` (official image has no arm64; Apple Silicon runs it under emulation). Do not use `postgres:16-alpine` (no PostGIS binaries).

**Existing old volume (plain postgres image):** PostGIS cannot be installed into that data directory; recreate:

```bash
make docker-down
docker volume rm lowcode-database_pg-data   # or the compose project volume name
make docker-up
```

Or, without deleting data, after switching to the PostGIS image:

```bash
docker exec -it lowcode-postgres psql -U postgres -d lowcode_data -c 'CREATE EXTENSION IF NOT EXISTS postgis;'
```

(If the image is still `postgres:16-alpine`, that SQL fails.)

### Self-hosted PostgreSQL (Linux)

Debian/Ubuntu example:

```bash
sudo apt install postgresql-16-postgis-3
sudo systemctl restart postgresql
```

RHEL / Amazon Linux: install the matching `postgis` / `postgis33_16` package (name varies).

Then on each data DB:

```sql
\c lowcode_data
CREATE EXTENSION IF NOT EXISTS postgis;
SELECT PostGIS_Version();
```

### Managed cloud

| Platform | Approach |
|----------|----------|
| **AWS RDS / Aurora** | Parameter group allows `postgis`; `CREATE EXTENSION postgis;` (some regions need `postgis_raster` etc.) |
| **GCP Cloud SQL** | Enable the PostGIS flag or use a PostGIS-capable version; then `CREATE EXTENSION postgis;` |
| **Azure Database for PostgreSQL** | Allow-list `POSTGIS`; `CREATE EXTENSION postgis;` |
| **Supabase / Neon** | Console or SQL editor: `CREATE EXTENSION postgis;` (if the plan supports it) |

### Multi-tenant: enable on every data database

`lowcode_data` is only the default tenant DB. When Admin creates a tenant with a **dedicated database**, the same extensions run via `EnsureDataTables` on that DSN.

### Verify

```bash
psql "$DATA_DATABASE_URL" -c "SELECT PostGIS_Version();"
```

API column types: `geometry`, `geography`, `point` (see `internal/columntype/types.go`).
