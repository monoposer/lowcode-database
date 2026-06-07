# api / apiv1

HTTP transport and JSON contract types. **No business logic.**

## internal/api

| Path | Role |
|------|------|
| `routes.go` | All route registration (chi) |
| `httputil/base.go` | JSON, tenant context, read-only tenant intercept |
| `admin/platform.go` | Tenants, connection, API Key, types, schema-audit |
| `admin/schema.go` | Tables/columns/indexes/Domain/relations/Query |
| `data/row.go` | Row CRUD, query, bulk, import/export; execute saved Query |
| `openapi/` | OpenAPI 3 + Swagger UI |

### API planes

| Prefix | Role |
|--------|------|
| `/v1/admin/*` | Schema changes, tenants, observability (**Meta API**) |
| `/v1/data/*` | Row I/O, execute saved Query |
| `/v1/worker/*` | calc_queue drain/claim/ack (**Worker API**) |

Control (`cmd/server`) middleware: CORS → RequestLog → **authn** → `NewHandler` (admin + data + calc HTTP).

## internal/apiv1

Hand-written JSON types, **no protobuf**.

| Subpackage | Types |
|------------|-------|
| *(root)* | `Value`, `SortOrder`, cell conversion |
| `schema/` | Table, Column, Index, **Domain**, Relation, ER |
| `row/` | Row, CRUD/import/export |
| `query/` | Query admin + execute (HTTP `/queries`) |
| `platform/` | Tenant, API Key |

Handler pattern: `admin.Tables{Base}` → `h.Svc.CreateTable(...)` → `LowcodeService` embedded domain methods.

## Conventions

- New endpoints: register in `routes.go` + `apiv1` types + service domain method.
- Header **`X-Tenant-Id`** is required (middleware writes it into context).
