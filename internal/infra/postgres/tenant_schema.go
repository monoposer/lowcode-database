package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/internal/config"
)

func (m *TenantManager) ensureTenantDataSchema(ctx context.Context, tenantID string, pool *pgxpool.Pool) error {
	if m == nil || pool == nil {
		return nil
	}
	tables, err := m.DataTablesForTenant(ctx, tenantID)
	if err != nil {
		return EnsureVirtualRecordsParent(ctx, pool)
	}
	return EnsureDataTables(ctx, pool, tables)
}

// TenantIsolationMode always returns virtual_records.
func (m *TenantManager) TenantIsolationMode() config.TenantIsolationMode {
	return config.TenantIsolationRLSTable
}

// TenantDataSchemaPrefix is unused in virtual_records (kept for Base compat).
func (m *TenantManager) TenantDataSchemaPrefix() string {
	if m == nil || m.tenantDataSchemaPrefix == "" {
		return config.DefaultTenantDataSchemaPrefix
	}
	return m.tenantDataSchemaPrefix
}
