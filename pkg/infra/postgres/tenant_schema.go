package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
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
