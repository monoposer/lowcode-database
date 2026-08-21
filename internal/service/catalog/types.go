package catalog

import (
	"context"

	"github.com/monoposer/lowcode-database/internal/columntype"
)

// ListTypes returns built-in pgTypes plus tenant columnTypes from meta DB.
// Builtin names cannot be overridden by tenant columnTypes.
func (s *Catalog) ListTypes(ctx context.Context) ([]*Type, error) {
	var types []*Type
	builtin := map[string]struct{}{}
	for _, t := range columntype.List() {
		refKind := columntype.RefKindPgType
		if t.Kind != "" {
			refKind = columntype.RefKindVirtual
		}
		builtin[t.ID] = struct{}{}
		types = append(types, &Type{
			Id: t.ID, Name: t.Name, PgType: t.PgType, Config: t.Config, RefKind: refKind,
		})
	}

	tenantCTs, err := s.ListColumnTypes(ctx)
	if err != nil {
		return nil, err
	}
	for _, ct := range tenantCTs {
		if _, ok := builtin[ct.Name]; ok {
			continue
		}
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
		if ct.Spec.FinancialMode != nil {
			cfg["financialMode"] = *ct.Spec.FinancialMode
		}
		if ct.Spec.RoundingMode != "" {
			cfg["roundingMode"] = ct.Spec.RoundingMode
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
