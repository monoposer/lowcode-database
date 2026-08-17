# Tenant isolation

Isolation unit is the **tenant** (`X-Tenant-Id` / `tenants.tenant_id`). All meta rows and business rows carry `tenant_id`. A tenant does not span data shards.

Related: [system.md](system.md) · [virtual-records.md](virtual-records.md) · [analysis.md](analysis.md)

---

## 1. Dual database

| DB | Connection | Contents |
|----|------------|----------|
| **Meta** | `META_DATABASE_URL` | `tenants`, `lc_bases`, `lc_tables` / `lc_columns` / `lc_indexes` / `lc_queries`, API keys, webhooks |
| **Data** | `tenants.data_dsn` | `record` (or `{tenant_id}_record`), `link_ref`, `calc_queue` |

```
HTTP + X-Tenant-Id
    → MetaPool: tenants.data_dsn
    → DataPool(ctx)  (keyed by DSN, shared by tenants on the same shard)
```

| Concept | Stored where | Decides |
|---------|--------------|---------|
| **DSN** | `tenants.data_dsn` (`data_dsn_write` / `data_dsn_reads`) | Which Postgres host and database |
| **Pool** | In-process `TenantManager.dataPools` | TCP reuse; **one pool per DSN**, not per tenant |
| **Row parent** | `tenants.record_store` | Shared `record` vs `{tenant_id}_record` on that DSN |

There is no `search_path` switching. SQL uses qualified table names. Static queries must include `tenant_id` (and `base_id` on meta `lc_*`).

---

## 2. Hierarchy

```
Tenant (tenant_id)  ──data_dsn──►  Data DB
    └── Base (base_id)
            └── Table (name = API Table.Id, row key = vt_id)
                    └── Column
```

`vt_id` (`lc_tables.vt_id`) is globally unique. Rows for a logical table are `record` rows with that `vt_id`. Multiple tenants may share one data DSN; they never share a `vt_id`.

---

## 3. Record store

`POST /v1/admin/tenants` field `recordStore`: `"shared"` (default) or `"dedicated"`. Changing it after create is not supported.

| `record_store` | Physical tables on the tenant data DSN |
|----------------|----------------------------------------|
| `shared` | `record`, `link_ref`, `calc_queue` — all tenants on the DSN, filtered by `tenant_id` |
| `dedicated` | `{tenant_id}_record`, `{tenant_id}_link_ref`, `{tenant_id}_calc_queue` |

A **private Postgres** is orthogonal: set `data_dsn` to that database. Table names still follow `record_store` (usually `shared` on a private DB).

```sql
-- migrations/meta/000001_init.up.sql
CREATE TABLE tenants (
    tenant_id      TEXT PRIMARY KEY,
    data_dsn       TEXT NOT NULL,
    data_dsn_write TEXT NOT NULL DEFAULT '',
    data_dsn_reads TEXT[] NOT NULL DEFAULT '{}',
    record_store   TEXT NOT NULL DEFAULT 'shared',
    ...
    CONSTRAINT tenants_record_store_check CHECK (record_store IN ('shared', 'dedicated'))
);
```

```json
POST /v1/admin/tenants
{
  "id": "acme",
  "displayName": "Acme",
  "dataDsn": "postgresql://app:secret@db:5432/lowcode_data",
  "recordStore": "shared"
}
```

Omit `dataDsn` to use `DATA_DSN_TEMPLATE`. `createDatabase: true` needs `DATA_ADMIN_DATABASE_URL`.
CreateTenant also seeds a **public** base (`name=public`, `baseId=base_{tenantId}`) and a default API key (plaintext returned once as `key`).

---

## 4. Pools (`pkg/infra/postgres`)

| Item | Behavior |
|------|----------|
| Library | pgx/v5 `pgxpool` |
| Meta pool | One per process, created at startup |
| Data pools | Lazy, keyed by write/read **DSN** |
| Read pool | `data_dsn_reads` round-robin; `consistency=strong` uses primary |
| Eviction | LRU beyond `MAX_TENANT_DATA_POOLS` |
| Per-tenant cap | `tenants.pool_max_conns` > 0 overrides `PG_MAX_CONNS` |

Size Postgres `max_connections` from unique DSNs × `PG_MAX_CONNS`, plus the meta pool — not from tenant count.

Code: `TenantManager` (`MetaPool` / `DataPool` / `DataReadPool`), `pkg/infra/postgres/record_store.go` (`ResolveDataTables`).
