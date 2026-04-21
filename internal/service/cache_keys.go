package service

import (
	"context"
	"fmt"
)

func cacheKeyDataSource(tenantID, dsID string) string {
	return fmt.Sprintf("lc:meta:ds:%s:%s", tenantID, dsID)
}

func cacheKeyColumns(tenantID, tableName string) string {
	return fmt.Sprintf("lc:meta:cols:%s:%s", tenantID, tableName)
}

func (s *LowcodeService) invalidateDataSourceCache(ctx context.Context, dsID string) {
	if s.cache == nil {
		return
	}
	tid, err := s.tenantID(ctx)
	if err != nil {
		return
	}
	_ = s.cache.Delete(ctx, cacheKeyDataSource(tid, dsID))
}

func (s *LowcodeService) invalidateTableMetaCache(ctx context.Context, tableName string) {
	if s.cache == nil {
		return
	}
	tid, err := s.tenantID(ctx)
	if err != nil {
		return
	}
	_ = s.cache.Delete(ctx, cacheKeyColumns(tid, tableName))
}

type cachedColumnMetaBundle struct {
	Cols       []fullColumnMeta
	SchemaName string
	TableName  string
}
