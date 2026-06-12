package catalog

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"strings"
)

func (s *Catalog) choiceToAPI(ctx context.Context, tid, schemaName, pgTypeName string) (*apiv1schema.Choice, error) {
	logicalName, err := choiceLogicalNameFromPgType(tid, pgTypeName)
	if err != nil {
		return nil, err
	}
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	values, err := s.listPgEnumValues(ctx, data, schemaName, pgTypeName)
	if err != nil {
		return nil, err
	}
	return &apiv1schema.Choice{
		Id:         logicalName,
		Name:       logicalName,
		Label:      logicalName,
		SchemaName: schemaName,
		PgTypeName: logicalName,
		Values:     values,
	}, nil
}

// ResolveChoiceValues reads enum labels from PostgreSQL pg_enum.
func (s *Catalog) ResolveChoiceValues(ctx context.Context, choiceName string) ([]*apiv1schema.ChoiceItem, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	schema, pgType, err := s.ResolveChoicePgTypeRef(ctx, tid, choiceName)
	if err != nil {
		return nil, err
	}
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	return s.listPgEnumValues(ctx, data, schema, pgType)
}

func (s *Catalog) ResolveChoicePgType(ctx context.Context, tid, choiceRef string) (schemaName, typeName string, err error) {
	return s.ResolveChoicePgTypeRef(ctx, tid, choiceRef)
}

func choiceRefFromConfig(cfg map[string]any) string {
	if ref := shared.CfgString(cfg, "choice_id"); ref != "" {
		return ref
	}
	return shared.CfgString(cfg, "choice_name")
}

// ResolveChoiceColumnRef decides if a column uses a PG ENUM (Choice).
// type_id should be the choice logical name; config.choice_name is optional legacy.
func (s *Catalog) ResolveChoiceColumnRef(ctx context.Context, tid, typeID string, cfg map[string]any) (logicalName string, ok bool, err error) {
	if typeID == "enum" {
		return "", false, fmt.Errorf("type_id %q is not valid; use the choice logical name as typeId (see POST /v1/choices)", typeID)
	}
	if ref := choiceRefFromConfig(cfg); ref != "" {
		if _, _, e := s.ResolveChoicePgType(ctx, tid, ref); e != nil {
			return "", false, e
		}
		return ref, true, nil
	}
	if _, _, e := s.ResolveChoicePgType(ctx, tid, typeID); e == nil {
		return typeID, true, nil
	}
	return "", false, nil
}

func (s *Catalog) ChoiceColumnDDLType(ctx context.Context, tid, choiceRef string) (string, error) {
	enumSchema, enumType, err := s.ResolveChoicePgType(ctx, tid, choiceRef)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s",
		pgx.Identifier{enumSchema}.Sanitize(),
		pgx.Identifier{enumType}.Sanitize(),
	), nil
}

// ColumnPgTypeSQL returns PostgreSQL type SQL for a column (built-in or choice).
func (s *Catalog) ColumnPgTypeSQL(ctx context.Context, tid, typeID string, cfg map[string]any) string {
	if columntype.Kind(typeID) == "relation_fk" {
		if pg, err := s.B.RelationFKColumnPgType(ctx, tid, cfg); err == nil && pg != "" {
			return pg
		}
	}
	if columntype.IsBuiltIn(typeID) {
		return shared.EffectivePgType(columntype.PgType(typeID), columntype.Config(typeID))
	}
	ref := typeID
	if typeID == "enum" {
		ref = choiceRefFromConfig(cfg)
	}
	if ref == "" {
		return ""
	}
	ddl, err := s.ChoiceColumnDDLType(ctx, tid, ref)
	if err != nil {
		return ""
	}
	return ddl
}

func (s *Catalog) CreateChoice(ctx context.Context, req *apiv1schema.CreateChoiceRequest) (*apiv1schema.CreateChoiceResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	pgTypeName, err := choicePgTypeName(tid, req.Name)
	if err != nil {
		return nil, err
	}
	literals, err := enumValuesFromItems(req.Values)
	if err != nil {
		return nil, err
	}
	schemaName := "public"
	if req.SchemaName != "" {
		schemaName, err = sanitizePgIdent(req.SchemaName)
		if err != nil {
			return nil, err
		}
	}

	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := data.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, pgx.Identifier{schemaName}.Sanitize())); err != nil {
		return nil, err
	}
	if err := s.createPgEnumType(ctx, data, schemaName, pgTypeName, literals); err != nil {
		return nil, fmt.Errorf("create pg enum: %w", err)
	}

	ch, err := s.choiceToAPI(ctx, tid, schemaName, pgTypeName)
	if err != nil {
		return nil, err
	}
	if req.Label != "" {
		ch.Label = req.Label
	}
	return &apiv1schema.CreateChoiceResponse{Choice: ch}, nil
}

func (s *Catalog) ListChoices(ctx context.Context, _ *apiv1schema.ListChoicesRequest) (*apiv1schema.ListChoicesResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	data, err := s.B.Tenants.DataReadPool(ctx)
	if err != nil {
		return nil, err
	}
	enums, err := s.listTenantChoiceEnums(ctx, data, tid)
	if err != nil {
		return nil, err
	}
	var resp apiv1schema.ListChoicesResponse
	for _, e := range enums {
		ch, err := s.choiceToAPI(ctx, tid, e.Schema, e.TypeName)
		if err != nil {
			return nil, err
		}
		resp.Choices = append(resp.Choices, ch)
	}
	return &resp, nil
}

func (s *Catalog) GetChoice(ctx context.Context, req *apiv1schema.GetChoiceRequest) (*apiv1schema.GetChoiceResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	schema, pgType, err := s.ResolveChoicePgTypeRef(ctx, tid, req.Id)
	if err != nil {
		return nil, err
	}
	ch, err := s.choiceToAPI(ctx, tid, schema, pgType)
	if err != nil {
		return nil, err
	}
	return &apiv1schema.GetChoiceResponse{Choice: ch}, nil
}

func (s *Catalog) UpdateChoice(ctx context.Context, req *apiv1schema.UpdateChoiceRequest) (*apiv1schema.UpdateChoiceResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	schema, pgType, err := s.ResolveChoicePgTypeRef(ctx, tid, req.Id)
	if err != nil {
		return nil, err
	}
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.Values) > 0 {
		literals, err := enumValuesFromItems(req.Values)
		if err != nil {
			return nil, err
		}
		if req.ReplaceValues {
			if err := s.replacePgEnumValues(ctx, data, schema, pgType, literals); err != nil {
				return nil, fmt.Errorf("replace enum values: %w", err)
			}
		} else if err := s.addPgEnumValues(ctx, data, schema, pgType, literals); err != nil {
			return nil, fmt.Errorf("add enum values: %w", err)
		}
	}
	ch, err := s.choiceToAPI(ctx, tid, schema, pgType)
	if err != nil {
		return nil, err
	}
	if req.Label != "" {
		ch.Label = req.Label
	}
	return &apiv1schema.UpdateChoiceResponse{Choice: ch}, nil
}

func (s *Catalog) DeleteChoice(ctx context.Context, req *apiv1schema.DeleteChoiceRequest) (*apiv1schema.DeleteChoiceResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	schema, pgType, err := s.ResolveChoicePgTypeRef(ctx, tid, req.Id)
	if err != nil {
		return nil, err
	}
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.dropPgEnumType(ctx, data, schema, pgType); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "depends on") {
			return nil, fmt.Errorf("enum type is in use by a column; drop column first")
		}
		return nil, err
	}
	return &apiv1schema.DeleteChoiceResponse{}, nil
}
