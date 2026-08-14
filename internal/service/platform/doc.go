// Package platform implements tenant provisioning, API keys, query admin,
// and observability list APIs.
//
//   - tenant.go — CreateTenant / ListTenants
//   - base.go — ListBases / CreateBase
//   - connection.go — database connection info
//   - apikey.go — API key CRUD
//   - query.go — lc_queries admin
//   - pgstat.go — pg_stat_statements list (PG_STAT_STATEMENTS)
//
// Type catalog (ListTypes) lives in catalog/types.go.
//
// Docs: docs/modules/platform.md
package platform
