package shared

import (
	"context"
	"fmt"
)

func CacheKeyDataSource(tenantID, tableID, dsName string) string {
	return fmt.Sprintf("lc:meta:ds:%s:%s:%s", tenantID, tableID, dsName)
}

func CacheKeyColumns(tenantID, tableName string) string {
	return fmt.Sprintf("lc:meta:cols:%s:%s", tenantID, tableName)
}

func (b *Base) InvalidateDataSourceCache(ctx context.Context, tableID, dsName string) {
	if b.Cache == nil {
		return
	}
	tid, err := b.TenantID(ctx)
	if err != nil {
		return
	}
	_ = b.Cache.Delete(ctx, CacheKeyDataSource(tid, tableID, dsName))
}

func (b *Base) InvalidateTableMetaCache(ctx context.Context, tableName string) {
	if b.Cache == nil {
		return
	}
	tid, err := b.TenantID(ctx)
	if err != nil {
		return
	}
	_ = b.Cache.Delete(ctx, CacheKeyColumns(tid, tableName))
}
