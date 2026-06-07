# platform module

**Path:** `internal/service/platform`  
**Role:** Tenants and connections, API Key, Query admin, observability lists.

## Core files

| File | Role |
|------|------|
| `tenant.go` | `CreateTenant` |
| `connection.go` | `GET /database/connection` |
| `apikey.go` | API Key CRUD |
| `query.go` | `lc_queries` admin |
| `admin_observability.go` | schema audit |

## Not in this module

| Capability | Actual module |
|------------|---------------|
| `ListTypes` | **catalog** (`types.go`) |
| Domain | **catalog** |
| Table/column DDL | **schema** |

## Admin API

| Path | Notes |
|------|-------|
| `POST /v1/admin/tenants` | Register `tenants` + schema provisioning (`recordStore`: `shared` \| `dedicated`) |
| `/v1/admin/api-keys` | Auth keys |
| `/v1/admin/queries` | Saved query views |

## Dependencies

- `infra/postgres.TenantManager` — tenants and connection pools

Post-create tenant wiring: Admin APIs (`CreateTable`, `AddColumn`, …). Schema bundle import is [roadmap](../roadmap.md#schema-bundle-import).
