# meta / shared modules

## meta — cross-domain read facade

**Path:** `internal/service/meta`

`LowcodeService` does **not** embed meta; use `meta.New(base)` or internal `data/meta()` as needed.

| Role | Delegates to |
|------|----------------|
| LoadColumns | catalog |
| Index list | catalog |
| Column PG types | catalog |

Avoids schema/data import cycles. **Writes** still go through schema/catalog/data.

## shared — shared kernel

**Path:** `internal/service/shared`

| File / area | Role |
|-------------|------|
| `base.go` | `Base`: TenantManager, Cache, Metrics, isolation mode |
| `events.go` | `EmitEvent`: `metadata.*` → `lc_schema_audit` (no webhook delivery) |
| `helpers.go` | Value conversion, TenantID, ResolveDataSchema |
| `config.go` | Virtual column config validation (link, rollup) |
| `result_type.go` | formula/lookup/rollup result types |
| `virtual.go` | EffectivePgType, formula SQL fragments |
| `cells.go` | Cell reference resolution |

### Base injection (`lowcode_service.go`)

```go
service.WithCache(...)
service.WithPGStatStatements(...)
service.WithLogger(...)
```

Every domain module holds `*shared.Base`. There are **no** direct struct references between domains.

## Dependency rules

```
schema / catalog / data / platform
        ↓
      shared.Base
        ↓
   infra · event · platform/cache ...
```

`meta` is read-only: no EmitEvent, no DDL.
