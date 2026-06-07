package tenant

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/monoposer/lowcode-database/internal/config"
)

var pgSchemaIdentRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

// ReservedDataSchemas are PostgreSQL system schemas that must not hold tenant business data.
var ReservedDataSchemas = map[string]struct{}{
	"pg_catalog":         {},
	"information_schema": {},
	"pg_toast":           {},
}

// DataSchemaName returns the default PostgreSQL schema for tenant business objects.
// RLSTableSchemaMarker is stored in lc_tables.schema_name when TENANT_ISOLATION_MODE=rls_table.
const RLSTableSchemaMarker = "_rls"

func DataSchemaName(tenantID, prefix string, mode config.TenantIsolationMode) string {
	if mode == config.TenantIsolationRLSTable {
		return RLSTableSchemaMarker
	}
	if mode == config.TenantIsolationSharedDB {
		if prefix == "" {
			prefix = config.DefaultTenantDataSchemaPrefix
		}
		name := prefix + SanitizeForSchema(tenantID)
		if len(name) > 63 {
			name = name[:63]
		}
		if !pgSchemaIdentRe.MatchString(name) {
			return prefix + "x_" + SanitizeForSchema(tenantID)
		}
		return name
	}
	return "public"
}

// SanitizeForSchema converts a tenant id into a PG-safe schema name suffix (no prefix).
func SanitizeForSchema(tenantID string) string {
	s := strings.TrimSpace(tenantID)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	if s == "" {
		return "unknown"
	}
	outline := regexp.MustCompile(`[^a-zA-Z0-9_]+`).ReplaceAllString(s, "_")
	if outline == "" {
		return "unknown"
	}
	if len(outline) > 50 {
		outline = outline[:50]
	}
	return outline
}

// ValidateDataSchema checks whether schemaName may hold tenant business data.
func ValidateDataSchema(schemaName, tenantID, prefix string, mode config.TenantIsolationMode) error {
	schemaName = strings.TrimSpace(schemaName)
	if schemaName == "" {
		return fmt.Errorf("schema name is required")
	}
	if !pgSchemaIdentRe.MatchString(schemaName) {
		return fmt.Errorf("schema name %q is not a valid PostgreSQL identifier", schemaName)
	}
	lower := strings.ToLower(schemaName)
	if strings.HasPrefix(lower, "pg_") {
		return fmt.Errorf("schema %q is reserved (PostgreSQL system namespace)", schemaName)
	}
	if _, reserved := ReservedDataSchemas[lower]; reserved {
		return fmt.Errorf("schema %q is reserved for system use", schemaName)
	}
	if mode == config.TenantIsolationSharedDB {
		expected := DataSchemaName(tenantID, prefix, mode)
		if schemaName != expected {
			return fmt.Errorf("shared_db mode: tenant data must use schema %q (got %q)", expected, schemaName)
		}
	}
	if mode == config.TenantIsolationRLSTable {
		if schemaName != RLSTableSchemaMarker && schemaName != "" {
			return fmt.Errorf("rls_table mode: tenant data uses shared store (schema %q not allowed)", schemaName)
		}
	}
	return nil
}

// ResolveDataSchema picks the PG schema for DDL/DML: explicit override or mode default.
func ResolveDataSchema(explicit, tenantID, prefix string, mode config.TenantIsolationMode) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		if err := ValidateDataSchema(explicit, tenantID, prefix, mode); err != nil {
			return "", err
		}
		return explicit, nil
	}
	schema := DataSchemaName(tenantID, prefix, mode)
	if err := ValidateDataSchema(schema, tenantID, prefix, mode); err != nil {
		return "", err
	}
	return schema, nil
}
