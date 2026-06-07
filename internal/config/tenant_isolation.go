package config

import "strings"

// TenantIsolationMode is retained for a few call sites / tests.
// Storage is always Virtual-Records (tenant shards + LIST-partitioned virtual_records).
// dedicated_db / shared_db are no longer selectable.
type TenantIsolationMode string

const (
	// Deprecated: physical per-tenant DB mode is removed.
	TenantIsolationDedicatedDB TenantIsolationMode = "dedicated_db"
	// Deprecated: shared schema mode is removed.
	TenantIsolationSharedDB TenantIsolationMode = "shared_db"
	// TenantIsolationRLSTable is the only active mode (virtual_records).
	TenantIsolationRLSTable TenantIsolationMode = "virtual_records"
)

// ParseTenantIsolationMode always returns virtual_records. The raw env value is ignored.
func ParseTenantIsolationMode(raw string) TenantIsolationMode {
	_ = strings.TrimSpace(raw)
	return TenantIsolationRLSTable
}

// DefaultTenantDataSchemaPrefix is unused in virtual_records (kept for API compat).
const DefaultTenantDataSchemaPrefix = "tenant_"
