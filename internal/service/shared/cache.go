package shared

import (
	"context"
	"fmt"
)

func CacheKeyQuery(tenantID, tableID, queryName string) string {
	return fmt.Sprintf("lc:meta:query:%s:%s:%s", tenantID, tableID, queryName)
}

func CacheKeyColumns(tenantID, tableName string) string {
	return fmt.Sprintf("lc:meta:cols:%s:%s", tenantID, tableName)
}

// invalidate is the only path that may delete cache keys. Domain code must use
// InvalidateQueryCache / InvalidateTableMetaCache instead of Cache.Delete.
func (b *Base) invalidate(ctx context.Context, keys ...string) {
	if b == nil || b.Cache == nil || len(keys) == 0 {
		return
	}
	_ = b.Cache.Delete(ctx, keys...)
}

func (b *Base) InvalidateQueryCache(ctx context.Context, tableID, queryName string) {
	tid, err := b.TenantID(ctx)
	if err != nil {
		return
	}
	b.invalidate(ctx, CacheKeyQuery(tid, tableID, queryName))
}

func (b *Base) InvalidateTableMetaCache(ctx context.Context, tableName string) {
	tid, err := b.TenantID(ctx)
	if err != nil {
		return
	}
	b.invalidate(ctx, CacheKeyColumns(tid, tableName))
}
