# Documentation

English is the language of this repository’s docs. The only Chinese overview is the root [README.zh.md](../README.zh.md).

| Area | Description |
|------|-------------|
| [architecture/](architecture/system.md) | Runtime: processes, dual DB, shards, `record` / calc |
| [modules/](modules/README.md) | Code layout by domain package |
| [roadmap.md](roadmap.md) | Not in this version (import pack, plugins, graph, ENUM, RBAC, …) |
| [ops/production-checklist.md](ops/production-checklist.md) | Production deploy checklist (Phase-1) |

## Architecture

| Doc | Description |
|-----|-------------|
| [architecture/system.md](architecture/system.md) | **Current system**: processes, dual DB, pgx tenancy, record/calc, Playground |
| [architecture/analysis.md](architecture/analysis.md) | Layers, data flow, trade-offs |
| [architecture/tenant-isolation.md](architecture/tenant-isolation.md) | Isolation: current `record` path vs historical `dedicated_db` / `shared_db` |
| [architecture/virtual-records.md](architecture/virtual-records.md) | Tenant shards, LIST partitions, indexes (`record` evolved from `virtual_records`) |
| [architecture/record-calc.md](architecture/record-calc.md) | `record.data` jsonb + `link_ref` + `calc_queue` |

## Related

- [pkg/typespec](../pkg/typespec/README.md) — column types (pgType / columnType / DOMAIN)
- [deploy/](../deploy/RELEASE.md) — Docker deploy and release
- [AGENTS.md](../AGENTS.md) — agent / developer quick reference
