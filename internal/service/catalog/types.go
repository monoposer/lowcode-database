package catalog

import (
	"context"
	"github.com/monoposer/lowcode-database/internal/columntype"
)

// ListTypes returns built-in pgTypes plus tenant columnTypes from meta DB.
func (s *Catalog) ListTypes(ctx context.Context) ([]*Type, error) {
	var types []*Type
	for _, t := range columntype.List() {
		refKind := columntype.RefKindPgType
		if t.Kind != "" {
			refKind = columntype.RefKindVirtual
		}
		types = append(types, &Type{
			Id: t.ID, Name: t.Name, PgType: t.PgType, Config: t.Config, RefKind: refKind,
		})
	}
	cts, err := s.ListColumnTypes(ctx)
	if err != nil {
		return nil, err
	}
	for _, ct := range cts {
		types = append(types, columnTypeToAPIType(ct))
	}
	return types, nil
}

func columnTypeToAPIType(ct *ColumnTypeDef) *Type {
	cfg := map[string]any{"columnType": true}
	if ct.Spec != nil {
		cfg["pgType"] = ct.Spec.PgType
		if ct.Spec.Array {
			cfg["array"] = true
		}
		if ct.Spec.Precision != nil {
			cfg["precision"] = *ct.Spec.Precision
		}
		if ct.Spec.Scale != nil {
			cfg["scale"] = *ct.Spec.Scale
		}
	}
	pgType := ct.Name
	if ct.Spec != nil {
		if eff, err := columntype.EffectiveColumnTypePgType(*ct.Spec); err == nil {
			pgType = eff
		}
	}
	return &Type{
		Id: ct.Name, Name: ct.Label, Label: ct.Label,
		PgType: pgType, SchemaName: ct.SchemaName,
		Config: cfg, RefKind: columntype.RefKindColumnType,
	}
}
