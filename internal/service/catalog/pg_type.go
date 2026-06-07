package catalog

import (
	"context"

	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// ColumnPgTypeSQL returns PostgreSQL type SQL for a column (built-in pgType or columnType DOMAIN).
func (s *Catalog) ColumnPgTypeSQL(ctx context.Context, tid, typeID string, cfg map[string]any) string {
	_ = cfg
	if _, err := columntype.Resolve(typeID); err == nil {
		t, _ := columntype.Get(typeID)
		return shared.EffectivePgType(t.PgType, t.Config)
	}
	if ddl, err := s.ColumnTypeDDL(ctx, tid, typeID); err == nil && ddl != "" {
		return ddl
	}
	return ""
}
