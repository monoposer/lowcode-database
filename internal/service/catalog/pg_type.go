package catalog

import (
	"context"

	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// ColumnPgTypeSQL returns PostgreSQL type SQL for a column (built-in pgType or columnType).
// Array-ness comes from columnType Spec.Array, not from column config.
func (s *Catalog) ColumnPgTypeSQL(ctx context.Context, tid, typeID string, cfg map[string]any) string {
	if t, err := columntype.Resolve(typeID); err == nil {
		if t.Kind != "" {
			if pg := shared.VirtualColumnPgType(typeID, cfg); pg != "" {
				return pg
			}
		}
		return shared.EffectivePgType(t.PgType, t.Config)
	}
	ct, err := s.loadColumnType(ctx, tid, typeID)
	if err != nil || ct.Spec == nil {
		if ddl, err2 := s.ColumnTypeDDL(ctx, tid, typeID); err2 == nil && ddl != "" {
			return ddl
		}
		return ""
	}
	pg, err := columntype.EffectiveColumnTypePgType(*ct.Spec)
	if err != nil {
		return ""
	}
	return pg
}
