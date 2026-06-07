package catalog

import (
	"context"

	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/apiv1/platform"
	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/pkg/typespec"
)

// ListTypes returns built-in pgTypes plus tenant columnTypes from meta DB.
func (s *Catalog) ListTypes(ctx context.Context, _ *platform.ListTypesRequest) (*platform.ListTypesResponse, error) {
	var res platform.ListTypesResponse
	for _, t := range columntype.List() {
		refKind := typespec.RefKindPgType
		if t.Kind != "" {
			refKind = typespec.RefKindVirtual
		}
		res.Types = append(res.Types, &apiv1schema.Type{
			Id: t.ID, Name: t.Name, PgType: t.PgType, Config: t.Config, RefKind: refKind,
		})
	}
	ctResp, err := s.ListColumnTypes(ctx, &apiv1schema.ListColumnTypesRequest{})
	if err != nil {
		return nil, err
	}
	for _, ct := range ctResp.ColumnTypes {
		res.Types = append(res.Types, columnTypeToAPIType(ct))
	}
	return &res, nil
}

func columnTypeToAPIType(ct *apiv1schema.ColumnTypeDef) *apiv1schema.Type {
	cfg := map[string]any{"columnType": true}
	if ct.Spec != nil {
		cfg["pgType"] = ct.Spec.PgType
		if ct.Spec.Precision != nil {
			cfg["precision"] = *ct.Spec.Precision
		}
		if ct.Spec.Scale != nil {
			cfg["scale"] = *ct.Spec.Scale
		}
	}
	return &apiv1schema.Type{
		Id: ct.Name, Name: ct.Label, Label: ct.Label,
		PgType: ct.Name, SchemaName: ct.SchemaName,
		Config: cfg, RefKind: typespec.RefKindColumnType,
	}
}
