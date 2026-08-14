# api

HTTP transport layer. **No business logic.** JSON resource types live in the domain packages under `internal/service/`.

| Package | Role |
|---------|------|
| `internal/api` | chi routes, handlers, middleware (`httputil`); HTTP JSON envelopes |
| `internal/service/schema` | Table, Column, Relation, ER |
| `internal/service/catalog` | Index, ColumnTypeDef, Type |
| `internal/service/data` | Row + query/list/bulk/export params |
| `internal/service/platform` | Tenant, API Key, Query, connection, … |
| `internal/service/shared` | Cell `Value`, `SortOrder`, conversions |

URL prefix `/v1/` is the HTTP version.

## internal/api

| Path | Role |
|------|------|
| `routes.go` | All route registration (chi) |
| `httputil/base.go` | JSON, tenant context, read-only tenant intercept |
| `admin/platform.go` | Tenants, connection, API Key, types, webhooks |
| `admin/schema.go` | Tables/columns/indexes/columnType/relations/Query |
| `data/row.go` | Row CRUD, query, bulk, export; execute saved Query |
| `openapi/` | OpenAPI 3 + Swagger UI |

### API planes

| Prefix | Role |
|--------|------|
| `/v1/admin/*` | Schema changes, tenants, observability (**Meta API**) |
| `/v1/data/*` | Row I/O, execute saved Query |

Control (`cmd/server`) middleware: CORS → RequestLog → **authn** → `NewHandler` (admin + data). Calc runs in-process (`internal/service/calc`), not as HTTP.

## Handler pattern

Handlers decode JSON into the service domain type, call `h.Svc.…`, and wrap the entity in a small envelope (`{"table": t}`).

- New endpoints: register in `routes.go` + domain type in the matching `internal/service/<module>` + service method.
