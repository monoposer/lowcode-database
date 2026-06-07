# Architecture analysis (current)

This document describes **this repository after the single-process merge**: layering, dual DB, and trade-offs.

Runtime map: [system.md](system.md).

---

## Product position

A self-hostable **Postgres low-code table engine**: HTTP JSON (`/v1/*`), metadata-driven schema, rows on a per-tenant data shard. Isolation unit is the tenant (`X-Tenant-Id`).

**One long-running process.** `cmd/server` serves admin, data, and calc. `cmd/migrate` is a CLI only (not a runtime). There is no `cmd/worker` and no `PROCESS_MODE`.

| Surface | Prefix | Role |
|---------|--------|------|
| Admin / Meta | `/v1/admin/*` | Tenants, bases, tables/columns, indexes, saved Query, API keys, webhooks |
| Data | `/v1/data/*` | Row CRUD, DSL query, export, execute saved Query |
| Calc HTTP | `/v1/worker/*` | Optional remote calc drain/claim/ack (same process also polls `calc_queue`) |

---

## Layers

```
┌─────────────────────────────────────────────────────────┐
│  cmd/server   /v1/admin + /v1/data + /v1/worker         │
│               + calc poll + EventBus + IndexMigrate     │
│  cmd/migrate  one-shot SQL (not a service)              │
├─────────────────────────────────────────────────────────┤
│  Contract     internal/apiv1 (hand-written JSON)        │
├─────────────────────────────────────────────────────────┤
│  Application  schema / catalog / data / platform        │
├─────────────────────────────────────────────────────────┤
│  Helpers      dsl, query, formula, columntype, event    │
├─────────────────────────────────────────────────────────┤
│  Infra        TenantManager (write/read pools), redis   │
├─────────────────────────────────────────────────────────┤
│  Persistence  Meta DB  +  per-tenant Data shard         │
└─────────────────────────────────────────────────────────┘
```

HTTP: CORS → request log → optional API-key authn → chi `NewHandler`.

---

## Dual-DB data flow

```
  Admin schema / Query / tenant     ┌──────────────┐
                                    │   Meta DB    │  tenants, lc_bases,
                                    │              │  lc_tables, lc_columns,
                                    └──────┬───────┘  lc_queries, webhooks
                                           │ data_dsn / data_dsn_write
                                           │ data_dsn_reads[]
                                    ┌──────▼───────┐
  Row query (replica or primary)    │  Data shard  │  record, link_ref,
  Row write (primary)               │              │  calc_queue, INDEX
                                    └──────────────┘
```

- Writes and DDL use `DataPool` (write DSN).
- Row query / export / saved-query use `DataReadPool` (round-robin replicas). `X-Read-Consistency: strong` or `consistency=strong` forces primary.
- `EmitEvent` writes `metadata.*` to `lc_schema_audit` and publishes `records.*` / `schema.*` on EventBus (`memory` or Redis Stream). Webhooks consume the bus.

```bash
make run    # :8080  admin + data + calc
```

---

## Domain modules

| Package | Role |
|---------|------|
| `schema` / `catalog` / `platform` | DDL, tenants, Query definitions, webhooks |
| `data` | Row CRUD, DSL, saved-query execute, calc enqueue |
| `calc` | In-process poll of shared `calc_queue` per DSN plus each dedicated `{tenant_id}_calc_queue` |

Authz/RBAC, graph query, plugins, schema-bundle import: [roadmap.md](../roadmap.md).

---

## Two splits that are easy to confuse

| Split | What it is | Isolation you get |
|-------|------------|-------------------|
| **Meta vs Data** | Two Postgres databases (or two DSNs) | Credentials, backup, scaling of catalog vs rows |
| **Admin vs Data vs Calc URLs** | Prefixes on **one** HTTP server | None. Same process, same pools, same crash domain |

The product used to look like “control plane vs data plane processes”. After the merge, **only the database split is architectural**. `/v1/admin` vs `/v1/data` is module layout and API taste, not an operational boundary.

---

## Request paths

```
X-Tenant-Id [+ X-Api-Key] [+ X-Read-Consistency]
        │
        ▼
  authn (optional API key, tenant-scoped)
        │
        ├─ /v1/admin/*     MetaPool (+ DataPool for DDL / ENUM-less catalog)
        ├─ /v1/data write  Meta (vt_id, columns) → DataPool → calc_queue → EmitEvent
        ├─ /v1/data read   Meta (columns / saved query) → DataReadPool → record jsonb
        └─ /v1/worker/*    DataPool calc claim/ack (optional; cmd/server also polls)
```

Every row request still **reads Meta** (table → `vt_id`, column spec). Redis can cache column/query JSON; it cannot skip Meta on a cold process. The server therefore always holds Meta credentials **and** every tenant write/read DSN.

`EmitEvent` is **after** the write, and publish errors are discarded. Events are at-most-once / best-effort, not an outbox in the data transaction.

---

## Three consistency planes (independent)

| Plane | Mechanism | What a client can assume |
|-------|-----------|--------------------------|
| **User fields** | `record.version` optimistic lock on the **write** DSN | A successful PATCH is on primary |
| **Calc cache** | `calc_queue` + `_cache_status` in `record.data` | List may be stale; GET detail may live-compute and only enqueue |
| **Replica** | `data_dsn_reads` round-robin; `consistency=strong` → primary | Default query can miss a just-written row |

There is **no** cross-row transaction (rollup parent/child). Link writes `link_ref` and enqueues; lookup/rollup catch up later. Mixing replica reads with calc-cache lists is eventually consistent twice.

---

## Isolation model

```
tenant_id  →  tenants.data_dsn     (which Postgres)
           →  record.tenant_id     (which rows in a shared DB)
table      →  lc_tables.vt_id      (LIST partition)
```

Two tenants may share one DSN (shared_db style). Isolation is then **predicates** (`tenant_id` / `base_id` / `vt_id`), not a separate cluster. Dedicated isolation is a DSN convention, not a different code path. Static SQL is supposed to go through `postgres.Where` / `AndWhere`; DSL SQL is assembled at runtime and must not drop those keys.

---

## Scaling axes

| You add | What happens |
|---------|----------------|
| More **tenants on one DSN** | Same pool; more `vt_id` partitions; Meta rows grow |
| More **unique DSNs** | More pgx pools (capped by `MAX_TENANT_DATA_POOLS`, LRU) |
| More **read replicas** | More pools; queries spread; writes unchanged |
| More **server replicas** | HTTP capacity ↑; **each** replica polls **every** shard `calc_queue`; memory EventBus no longer sufficient |
| Heavier **formula/lookup** | Same HTTP process; `SKIP LOCKED` fair-shares tasks, not CPU vs API |

The unit of scale for compute is “identical `cmd/server`”, not a worker fleet.

---

## Trust boundary

- **Authn**: optional `X-Api-Key` bound to a tenant. CORS also allows `X-User-Roles`; nothing in-process consumes them.
- **Authz**: `EnsureWritable` is a no-op. Any key for tenant T can DDL and dump T’s rows.
- **Secrets**: process memory holds Meta URL and all data DSNs (including replica passwords).

This is a **BFF-backed engine**, not a tenant-facing SaaS edge by itself.

---

## Strengths

1. **Operationally simple.** One binary, one port, one image. No server/worker version skew. Local `make run` is production-shaped.
2. **Postgres-native model.** Indexes are real PG indexes; virtual columns have no extra physical column; rows are `record` LIST partitions by `vt_id`.
3. **Clear tenancy.** `X-Tenant-Id` → Meta `tenants.data_dsn` → pooled connections keyed by DSN (shared across tenants on the same shard).
4. **Read/write split without a second process.** Replica routing lives in `TenantManager`; strong reads opt into the primary.
5. **Events are actually deliverable.** Memory bus for single instance; Redis Stream for multiple replicas of the same binary; webhooks replace polling `lc_schema_audit`.
6. **Query path is cheaper on repeats.** Process-local `dsl.ParseCached` and `BuildWhereWithTypesCached`; saved-query spec still Redis-cached.
7. **No gRPC/protobuf tax.** `apiv1` is the contract.

---

## Weaknesses and risks

1. **Single process is a blast radius.** Admin DDL, heavy export, and calc polling share CPU, memory, and the HTTP server. A stuck `calc_queue` or a large import can delay schema APIs. Horizontal scale means running **N identical all-in-one replicas**, which duplicates calc pollers unless you later add a leader lock.
2. **Calc fan-out on replicas.** Every instance polls every shard’s `calc_queue` (`SKIP LOCKED` makes this safe but wastes connections and wakeups). There is no elected calc owner.
3. **Memory EventBus does not cross instances.** Default `EVENT_BUS=memory` is local only. Multi-replica deploys must set `EVENT_BUS=redis` or webhooks will miss events from other processes. Redis Stream webhook consumer group delivers each event once — good — but schema audit is still per-writer to Meta.
4. **Replica lag is the caller’s problem.** Strong read is opt-in. A read-after-write without `consistency=strong` can miss the row on a lagging replica.
5. **Runtime DDL vs versioned SQL.** Data tables/partitions/indexes are created by the app; `migrations/data` only covers extensions (`cmd/migrate`). That does not version every tenant’s logical schema.
6. **Authz is external.** Any valid API key for the tenant can hit admin and data. Fine if a BFF enforces RBAC; see [roadmap](../roadmap.md#authorization-rbac).
7. **Product gaps.** Graph expand, plugins, Choice ENUM, schema-bundle import are out of scope; see [roadmap](../roadmap.md).
8. **Observability is thin.** In-process memory metrics, optional `pg_stat_statements`, JSON logs. No traces spanning Meta SQL → Data SQL → calc.
9. **Pool cardinality.** Write DSN + each replica DSN is a pool. Misconfigured `data_dsn_reads` (many unique hosts) multiplies `max_connections` pressure (`MAX_TENANT_DATA_POOLS` LRU-evicts).

---

## Evolution (if needed later)

| Need | Direction |
|------|-----------|
| Isolate calc CPU | Leader election (Redis) so only one replica polls `calc_queue`; others serve HTTP |
| Isolate admin vs data SLO | Re-introduce a **role flag on the same binary** (`-role=api\|calc`), not two codebases |
| Stronger events | Keep Redis Stream; add outbox in the data tx if “at least once after commit” is required |
| Direct internet exposure | External authz stays; optionally add a row-filter hook (not RBAC tables) |
| Meta isolation | Worker-style Meta HTTP client so data nodes do not hold Meta credentials |

Until those needs show up, **one process is the intended architecture**. Product gaps (import pack, plugins, graph, ENUM, RBAC) live in [roadmap.md](../roadmap.md).

---

## Comparison

| Dimension | lowcode-database | Typical ORM | Supabase-style BaaS |
|-----------|------------------|-------------|---------------------|
| Schema | Admin API + Meta | Code migrations | Dashboard + SQL |
| Query | Same process: Meta → DSN → SQL | App connects directly | PostgREST |
| Multi-tenant | Meta + per-tenant `data_dsn` | schema / RLS | Project-level |
| Process | Single HTTP + embedded calc | App servers | Many managed services |

---

## Related

- [system.md](system.md)
- [ops/production-checklist.md](../ops/production-checklist.md)
- [record-calc.md](record-calc.md)
- [roadmap.md](../roadmap.md)
