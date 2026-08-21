package catalog

import (
	"context"

	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/numeric"
)

// NumericSpecForType resolves financial numeric handling for a column type id.
func (s *Catalog) NumericSpecForType(ctx context.Context, tid, typeID string) (numeric.Spec, bool) {
	if t, err := columntype.Resolve(typeID); err == nil && t.PgType == "numeric" {
		return numeric.SpecFromConfig(t.Config), true
	}
	ct, err := s.loadColumnType(ctx, tid, typeID)
	if err != nil || ct.Spec == nil || ct.Spec.PgType != "number" {
		return numeric.Spec{}, false
	}
	return numeric.SpecFromColumnTypeSpec(ct.Spec), true
}
