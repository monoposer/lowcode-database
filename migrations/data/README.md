# Tenant data DB

Physical tables, indexes, and runtime DDL are created by the service (`internal/service/schema`, `catalog`).  
Versioned SQL is only for database-level extensions: `000001_postgis.up.sql`, `000002_pg_stat_statements.up.sql`.

## PostgreSQL version

Requires **PostgreSQL 16+**.

## Extensions

| Extension | Notes |
|-----------|-------|
| **pgcrypto** | Not needed. `gen_random_uuid()` is built-in since PG 13. |
| **postgis** | Required for `geometry` / `geography` / `point` columns. See below. |
| **pg_stat_statements** | SQL stats when `PG_STAT_STATEMENTS=true`. Needs `shared_preload_libraries=pg_stat_statements` (compose already sets this) + the `000002` migration. |

---

## Enabling PostGIS

PostGIS is **not** part of core PostgreSQL. Two steps:

1. **Install PostGIS software** (binaries and SQL scripts for the `postgis` extension)
2. **On each tenant data database** run `CREATE EXTENSION postgis;`

Missing either step, creating a `geometry` column fails with `type "geometry" does not exist` or `extension "postgis" is not available`.

### Local Docker (recommended)

`deploy/docker-compose.yml` uses **`postgis/postgis:16-3.5`** (not `postgres:16-alpine`).  
`make docker-up` only creates empty DBs; `make migrate` (or `make docker-migrate`) applies `000001_postgis.up.sql`.

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

`lowcode_data` is only the default tenant DB. When Admin creates a tenant with a **dedicated database**, run the same on that DSN:

```sql
CREATE EXTENSION IF NOT EXISTS postgis;
```

Automate this in tenant provisioning or the `DATA_ADMIN_DATABASE_URL` create-database flow.

### Verify

```bash
psql "$DEFAULT_TENANT_DATA_DSN" -c "SELECT PostGIS_Version();"
```

API column types: `geometry`, `geography`, `point` (see `internal/columntype/types.go`).
