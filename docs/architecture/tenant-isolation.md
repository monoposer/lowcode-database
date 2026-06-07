# Tenant isolation and schema strategy

> **Update:** storage is fixed to **Virtual-Records** (no `TENANT_ISOLATION_MODE`). Authoritative description: [virtual-records.md](virtual-records.md). `dedicated_db` / `shared_db` / old `rls_table` below are historical contrast only.

Current product path uses Virtual-Records. Isolation is three independent layers:

1. **Shared table** (`tenants.record_store = shared`, default): all tenants on a DSN use `record` / `link_ref` / `calc_queue`, filtered by `tenant_id`.
2. **Dedicated table** (`record_store = dedicated`): the same DSN, but physical tables `{tenant_id}_record`, `{tenant_id}_link_ref`, `{tenant_id}_calc_queue`.
3. **Dedicated database**: set `tenants.data_dsn` (already supported). Table names still follow `record_store` (usually `shared` on a private DB).

`POST /v1/admin/tenants` body field `recordStore`: `"shared"` | `"dedicated"`. Changing `recordStore` after create is not supported (rows would sit on the other parent).

Related: [system.md](system.md) · [analysis.md](analysis.md) · [virtual-records.md](virtual-records.md)

---

## 1. This repo vs Supabase schema-per-tenant

| Dimension | Supabase | lowcode-database (current) |
|-----------|----------|----------------------------|
| Metadata | `public.tenants` + tenant sandbox schema | **Dedicated Meta DB** (`lc_tables` etc., `tenant_id` column) |
| Business data | Many schemas in one DB: `tenant_{uuid}` | **Data DB**: see historical modes below |
| System schemas | `auth` / `storage` / `realtime` / `extensions` … | If Data DB shares a Supabase instance, those exist; **this service must not write them** |
| Routing | `SET LOCAL search_path = tenant_xxx, public` | This service uses **fully qualified** DDL/DML (`schema.table`); no search_path |
| Tenant identity | JWT `tenant_id` | HTTP **`X-Tenant-Id`** |

**Conclusion:** this service does not occupy Supabase system schemas. Under historical **shared_db**, tenant objects lived in `tenant_{id}`, namespaced away from `auth`/`storage`.

---

## 2. Historical isolation modes (removed; Virtual-Records only)

| Mode | Alias | Typical use |
|------|-------|-------------|
| `dedicated_db` | `dedicated` (**former default**) | Multi-DB: one database per tenant |
| `shared_db` | `shared` | Single DB: per-tenant `tenant_{id}` schema + physical tables |
| `rls_table` | `rls`, `single_table` | **Single-table tenant**: shared JSONB table + RLS |

### Mode A: `dedicated_db` (multi-database)

```
Meta DB (shared)
  lc_tenants: tenant_a → postgresql://.../db_a
              tenant_b → postgresql://.../db_b

Data DB tenant_a (db_a)          Data DB tenant_b (db_b)
  public.orders                    public.orders
  public.order_status (ENUM)       ...
```

| Item | Notes |
|------|-------|
| Env | `TENANT_ISOLATION_MODE=dedicated_db` (or omitted) |
| data_dsn | **Each tenant should use a different database** (`DATA_DSN_TEMPLATE` + `create_database`) |
| PG schema | Default **`public`**; `schemaName` optional on create |
| Fit | Enterprise, strong compliance, < 1500 tenants/DB, whole-DB backup |
| vs Supabase | Data DB can be **standalone Postgres** (no auth schema), or a **separate Project per tenant** |

**Pros:** strongest isolation; ENUM/table names need no prefix; pg_catalog split per DB.  
**Cons:** pool and database count grow with tenants; needs `DATA_ADMIN_DATABASE_URL` to auto-create DBs.

### Mode B: `shared_db` (one DB, many schemas)

```
Meta DB (shared)
  lc_tenants: tenant_a → postgresql://.../lowcode_data
              tenant_b → postgresql://.../lowcode_data  (same DSN)

Data DB (lowcode_data, may share a Supabase instance)
  tenant_a.orders          tenant_b.orders
  tenant_a.order_status    tenant_b.order_status
  public (platform/extensions only; this service writes no business tables)
  auth / storage / … (Supabase system; this service must not write)
```

| Item | Notes |
|------|-------|
| Env | `TENANT_ISOLATION_MODE=shared_db` |
| data_dsn | **Many tenants share one DSN** |
| PG schema | Automatic **`tenant_{sanitized_tenant_id}`** (prefix `TENANT_DATA_SCHEMA_PREFIX`) |
| Tables/ENUM | Forbidden on reserved schemas; no cross-tenant schema |
| Fit | Mid-size SaaS, hundreds–thousands of tenants, co-located with Supabase |
| Provisioning | `POST /v1/admin/tenants` or startup bootstrap `CREATE SCHEMA IF NOT EXISTS tenant_xxx` |

**Example:** tenant `7a2f9d4e-1234-4abc-8800-000000000001` → schema `tenant_7a2f9d4e_1234_4abc_8800_000000000001`

**Pros:** clear schema isolation in one DB; simple provisioning.  
**Cons:** pg_catalog pressure as schema count grows; **historical implementation cached pools by tenant id**, so N tenants on one `data_dsn` could hold N pgxpools.

### Mode C: `virtual_records` (former `rls_table`)

Unified row storage + tenant shards. See **[virtual-records.md](virtual-records.md)**.

```
Global-Meta
  pg_shards / tenants / lc_tables(vt_id, tenant_id) / lc_columns / …

PG Shard (routed by tenant_id)
  virtual_records PARTITION BY LIST (vt_id)
```

| Item | Notes |
|------|-------|
| Env | (none; only storage path) |
| Routing | `X-Tenant-Id` → `tenants.pg_instance_tag` / `data_dsn` → shard pool |
| Create table | Meta assigns global `vt_id` + `CREATE TABLE … PARTITION OF virtual_records` |
| Rows | `virtual_records` (`record_id`, `tenant_id`, `vt_id`, `data` JSONB) — implemented as `record` |
| Indexes / FTS | Index Migrate worker: `CREATE INDEX CONCURRENTLY` partial; `_fulltext_text` |
| Breaking | Old `lc_dynamic_rows` unused; data must be rebuilt |

---

## 3. `tenants` / `lc_tenants` and connection routing

### 3.1 Why `data_dsn`

The **Meta DB stores logical config** (table definitions, columns, views). **Physical rows and indexes** live in the Data DB. Each tenant must register **which Postgres to connect to** so the service can open the right Data pool.

Unlike Supabase mapping tenants to a `tenant_{uuid}` **schema**, the first routing hop is **DSN (host + database)**; PG schema is a second hop (historical `TENANT_ISOLATION_MODE`).

### 3.2 Meta table (connection fields)

```sql
-- migrations/meta/000001_init.up.sql
CREATE TABLE lc_tenants (
    id             TEXT PRIMARY KEY,
    display_name   TEXT NOT NULL DEFAULT '',
    data_dsn       TEXT NOT NULL,          -- write DSN
    read_dsn       TEXT NOT NULL DEFAULT '', -- optional read replica
    read_only      BOOLEAN NOT NULL DEFAULT FALSE,
    pool_max_conns INT NOT NULL DEFAULT 0, -- 0 → PG_MAX_CONNS
    ...
);
```

Business table **`lc_tables.schema_name`** records the PG schema inside the **connected Data DB** (`public` or `tenant_{id}`), combined with `data_dsn`.

### 3.3 How DSN was filled (historical)

| Mode | `data_dsn` | Connection lands on | Business storage |
|------|------------|---------------------|------------------|
| **dedicated_db** | Different **database** per tenant | Different databases | Physical `public.table` + PG ENUM |
| **shared_db** | Same database for many tenants | Same database | Per-tenant schema `tenant_*` + physical tables |
| **virtual_records** (`rls_table`) | Tenants split across shards | Shard selected by `tenant_id` | **`virtual_records`** LIST by `vt_id` + JSONB `data` |

**Admin create-tenant examples:**

```json
// dedicated_db: dedicated database
POST /v1/admin/tenants
{
  "id": "acme",
  "displayName": "Acme Corp",
  "dataDsn": "postgresql://app:secret@db.internal:5432/acme_data",
  "createDatabase": false
}

// dedicated_db: server creates the database (needs DATA_ADMIN_DATABASE_URL + DATA_DSN_TEMPLATE)
{
  "id": "beta",
  "createDatabase": true
}

// shared_db: shared Supabase / single DB (dataDsn optional; DEFAULT or same URL)
{
  "id": "shop-001",
  "dataDsn": "postgresql://postgres:...@db.supabase.co:5432/postgres"
}
```

With historical **`TENANT_ISOLATION_MODE=shared_db`**, create-tenant ran `CREATE SCHEMA IF NOT EXISTS tenant_shop_001` on that `data_dsn`.

### 3.4 Connection routing (orthogonal to isolation mode)

```
                    X-Tenant-Id
                         │
                         ▼
              ┌──────────────────────┐
              │  MetaPool (fixed)    │
              │  lookup tenants      │
              └──────────┬───────────┘
                         │ data_dsn / read_dsn
                         ▼
              ┌──────────────────────┐
              │  getOrCreatePool     │
              │  key = tenantId      │
              │      or tenantId:read│
              └──────────┬───────────┘
                         │
         ┌───────────────┴───────────────┐
         ▼                               ▼
  dedicated_db                      shared_db
  database A / B / …                same database
  schema often public               schema = tenant_*
         │                               │
         └───────────────┬───────────────┘
                         ▼
              DDL/DML: "schema"."table"
              (from lc_tables.schema_name)
```

**Do not confuse three addresses:**

| Concept | Stored where | Decides |
|---------|--------------|---------|
| **DSN** | `tenants.data_dsn` | Which Postgres host and **database** |
| **Pool** | In-process `TenantManager.dataPools` | TCP reuse to that DSN |
| **PG schema** | `lc_tables.schema_name` + historical mode defaults | Which **namespace** inside the DB |

### 3.5 Pool behavior (`internal/infra/postgres`)

| Item | Behavior |
|------|----------|
| Library | pgx/v5 `pgxpool` |
| Meta pool | Created at startup; one per process |
| Tenant Data pools | **Lazy by tenant id** historically; prefer **per-DSN reuse** in production |
| Read pool | Separate key `{id}:read` when `read_dsn` is set |
| Eviction | LRU close of least-used pools beyond `MAX_TENANT_DATA_POOLS` |
| Per-tenant cap | `tenants.pool_max_conns` > 0 overrides `PG_MAX_CONNS` |

**Ops (shared_db):** N tenants on one `data_dsn` historically could create **up to N** pgxpools to the same database (each `MaxConns` default 10). Size Postgres `max_connections` as `N × PG_MAX_CONNS + meta`, or share pools **by DSN**.

### 3.6 vs Supabase / Supavisor

| Deploy | Meta `data_dsn` points at | Notes |
|--------|---------------------------|-------|
| Pure lowcode-database | Self-hosted `lowcode_data` or per-tenant DBs | No Supabase schemas |
| Supabase + shared_db | Supabase Postgres **database URL** | Business in `tenant_*`; do not touch `auth`/`storage` |
| Supabase + dedicated_db | Per-tenant database or Project | Distinct DSNs, fully separate pools |
| Supavisor multi-tenant pool | Supavisor **transaction-mode** port | App-side + Supavisor-side pools **stack**; cap both |

This service **does not parse** Supabase JWT or `search_path`; Supavisor is just host/port in the DSN.

---

## 4. Reserved schema names

These names **must not** be used as tenant business schemas (create table / ENUM is rejected):

| Category | Schema |
|----------|--------|
| Postgres system | `pg_catalog`, `information_schema`, `pg_toast`; any `pg_` prefix (including `pg_temp_*` / `pg_toast_temp_*`) |

Under `shared_db`, tenants only used **`tenant_{id}`**. Under `dedicated_db`, default was `public`; explicit schemas still could not use reserved names.

---

## 5. Co-locating with Supabase

### 5.1 Name collisions: **no**

- System schemas: short English names (`auth`, `storage`)
- Tenant schemas: `tenant_` + sanitized tenant id
- Postgres: objects in different schemas are isolated; `auth.users` ≠ `tenant_xxx.orders`

### 5.2 search_path: **this service does not use it**

All DDL uses `"schema"."table"`. If PostgREST/Edge Functions share the connection, those components should `SET LOCAL search_path` themselves.

### 5.3 pg_catalog / information_schema growth: **yes, it stacks**

| Tenant scale (shared_db) | Effect |
|--------------------------|--------|
| < 500 | Barely noticeable |
| 500–3000 | `\d`, backups, VACUUM slow down |
| > 5000 | Split DBs or dedicated_db / mixed |

Each schema + table + ENUM occupies `pg_namespace` / `pg_class` / `pg_type`. Supabase already has hundreds of objects; **many `tenant_*` schemas worsen bloat**.

**Mitigations:**

- Cap schemas per DB (< 1500–2000)
- Multiple Supabase Projects / Postgres instances
- Large customers `dedicated_db`, small `shared_db` (mixed)
- Offboard: `DROP SCHEMA tenant_xxx CASCADE` (needs an ops script)

### 5.4 Migration impact

| Type | dedicated_db | shared_db |
|------|--------------|-----------|
| Meta migrations | `migrations/meta/` once | Same |
| Data business DDL | Runtime Admin API routed per tenant DB | Runtime into each `tenant_*` schema |
| Platform-wide structure | Per DB (changing physical columns needs per-tenant or external tools) | Cannot `ALTER` all tenant schemas in one SQL; loop or new-tenant templates only |

This service has **no versioned DDL** for Data business tables; tables/columns are created by API. DB-level extensions (PostGIS) live in `migrations/data/`.

---

## 6. Low-code / dynamic-form placement (historical)

| Need | dedicated_db | shared_db | rls_table |
|------|--------------|-----------|-----------|
| Per-tenant ENUM | ✅ PG ENUM | ✅ PG ENUM (in schema) | ✅ JSONB enum store |
| Independent table metadata | ✅ Meta + dedicated DB | ✅ Meta + tenant schema | ✅ Meta + `_rls` flag |
| pg_catalog pressure | Low (split by DB) | Grows with schemas | **Low** (fixed shared tables) |
| Complex SQL / indexes | ✅ Full | ✅ Full | ⚠️ v1 limited |
| Recommended tenants / DB | Enterprise / compliance | Hundreds–~1500 | **Tens of thousands** (JSONB size) |

### Config examples (historical)

**Local single tenant (former default):**

```bash
TENANT_ISOLATION_MODE=dedicated_db
META_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/lowcode_meta
DEFAULT_TENANT_DATA_DSN=postgresql://postgres:postgres@localhost:5432/lowcode_data
```

**Supabase same-DB multi-tenant (schema isolation):**

```bash
TENANT_ISOLATION_MODE=shared_db
```

**Mass tenants / dynamic forms (single-table JSONB):**

```bash
TENANT_ISOLATION_MODE=rls_table
TENANT_DATA_SCHEMA_PREFIX=tenant_
META_DATABASE_URL=postgresql://.../lowcode_meta
DEFAULT_TENANT_DATA_DSN=postgresql://.../postgres
```

**Enterprise multi-DB:**

```bash
TENANT_ISOLATION_MODE=dedicated_db
DATA_ADMIN_DATABASE_URL=postgresql://postgres:...@host/postgres
DATA_DSN_TEMPLATE=postgresql://postgres:...@host/%s
# POST /v1/admin/tenants { "createDatabase": true, ... }
```

---

## 7. Implementation notes (code)

| Component | Behavior |
|-----------|----------|
| `internal/config` | Historical `TENANT_ISOLATION_MODE`, `TENANT_DATA_SCHEMA_PREFIX` |
| `internal/tenant/schema.go` | Schema naming, reserved names, `ResolveDataSchema` |
| `internal/service/shared.Base` | `ResolveDataSchema(ctx, explicit)` |
| `CreateTable` | Default schema followed the mode |
| `internal/infra/postgres/rls_shared.go` | `lc_shared` + RLS + `WithRLSTenantSession` |
| `TenantManager` | `MetaPool` / `DataPool`; pgxpool cache |
| `TenantManager.CreateTenant` | Write `tenants`; shared_db created schema |

`lc_tables.schema_name` records the PG schema for each logical table.

---

## 8. Choice summary (historical)

| Metric | dedicated_db | shared_db | rls_table |
|--------|--------------|-----------|-----------|
| Per-tenant enum | ✅ PG ENUM | ✅ PG ENUM | ✅ JSONB store |
| pg_catalog bloat | Spread per DB | Grows with schemas | **Very low** |
| Complex query/index | ✅ | ✅ | ⚠️ v1 basic |
| Recommended tenants / DB | N/A | ~1500 | **Tens of thousands** (JSONB volume) |

Current product path is Virtual-Records / `record` only.

---

## Related

- [system.md](system.md)
- [analysis.md](analysis.md)
