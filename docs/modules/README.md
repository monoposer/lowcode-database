# Module index

lowcode-database organizes code and docs by **business domain**. Runtime architecture: [system.md](../architecture/system.md). Out of scope: [roadmap.md](../roadmap.md).

```
┌──────────── cmd ────────────┐
│ server · migrate            │
└──────────────┬──────────────┘
               ▼
┌──────────── api ──────────────┐
│ routes · admin/* · data/*     │
└──────────────┬────────────────┘
               ▼
┌────────────────── LowcodeService (+ domain JSON types) ──┐
│ schema │ catalog │ data │ platform │ shared.Value        │
└───┬────────┬───────┬─────────┬───────────────────────────┘
    │        │       │         │
    ▼        ▼       ▼         ▼
 meta    columntype  dsl     event
 shared              query
         formula
               │
               ▼
         infra (postgres · redis · tenant)
         platform-services (authn · cache · metrics)
```

---

## Business modules (`internal/service`)

| Module | Path | Doc | Role |
|--------|------|-----|------|
| **schema** | `internal/service/schema` | [schema.md](schema.md) | Table/column meta + physical DDL, virtual columns, Link projection |
| **catalog** | `internal/service/catalog` | [catalog.md](catalog.md) | Types, Domain, index catalog |
| **data** | `internal/service/data` | [data.md](data.md) | Row CRUD, `record`/`link_ref`, calc enqueue |
| **calc** | `internal/service/calc` | [record-calc.md](../architecture/record-calc.md) | formula/lookup/rollup queue and worker |
| **platform** | `internal/service/platform` | [platform.md](platform.md) | Tenants, API Key, Query admin |
| **meta** | `internal/service/meta` | [meta-shared.md](meta-shared.md) | Cross-domain metadata read facade |
| **shared** | `internal/service/shared` | [meta-shared.md](meta-shared.md) | Base, cells, events, result type |

---

## Transport

| Module | Path | Doc |
|--------|------|-----|
| **api** | `internal/api` | [api.md](api.md) |

JSON request/response types live in the matching `internal/service/<domain>` package (see table above).

---

## Engines

| Module | Path | Doc |
|--------|------|-----|
| **query-engine** | `dsl` · `query` · `formula` · `columntype` | [query-engine.md](query-engine.md) |
| **event** | `internal/event` | [event.md](event.md) |

Not in this version (plugins, graph query, Choice/ENUM, authz): [roadmap.md](../roadmap.md).

---

## Infrastructure

| Module | Path | Doc |
|--------|------|-----|
| **infra** | `pkg/infra` · `pkg/tenant` · `pkg/migrator` · `pkg/config` | [infra.md](infra.md) |
| **platform-services** | `pkg/platform` · `pkg/logger` · `pkg/telemetry` | [platform-services.md](platform-services.md) |
| **cmd** | `cmd/server` · `cmd/migrate` | [cmd.md](cmd.md) |

---

## Package → module map

| Path | Module doc |
|------|------------|
| `cmd/` | [cmd.md](cmd.md) |
| `internal/api` | [api.md](api.md) |
| `internal/service/schema` | [schema.md](schema.md) |
| `internal/service/catalog`, `internal/columntype` | [catalog.md](catalog.md) |
| `internal/service/data` | [data.md](data.md) |
| `internal/service/calc` | [record-calc.md](../architecture/record-calc.md) |
| `internal/service/platform` | [platform.md](platform.md) |
| `internal/service/meta`, `internal/service/shared` | [meta-shared.md](meta-shared.md) |
| `internal/dsl`, `internal/query`, `internal/formula` | [query-engine.md](query-engine.md) |
| `internal/event` | [event.md](event.md) |
| `pkg/infra`, `pkg/tenant`, `pkg/migrator`, `pkg/config` | [infra.md](infra.md) |
| `pkg/platform`, `pkg/logger`, `pkg/telemetry` | [platform-services.md](platform-services.md) |

---

## Conventions

1. **Domain logic** lives in `internal/service/<module>/`, exposed by embedding on `LowcodeService`.
2. **HTTP** only encodes/decodes JSON and routes; it does not write SQL.
3. **Meta / Data dual DB**: meta holds `lc_*` config; data holds physical tables, indexes, rows.
4. **Type resolution**: pgType → columnType (see [catalog.md](catalog.md)).
5. **Cross-domain reads** prefer `meta.Read`; handlers should not assemble SQL directly.
