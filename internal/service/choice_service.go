package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
)

// -------- Choice (PostgreSQL ENUM only; no meta table) --------

func (s *LowcodeService) choiceToAPI(ctx context.Context, tid, schemaName, pgTypeName string) (*apiv1.Choice, error) {
	logicalName, err := choiceLogicalNameFromPgType(tid, pgTypeName)
	if err != nil {
		return nil, err
	}
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	values, err := s.listPgEnumValues(ctx, data, schemaName, pgTypeName)
	if err != nil {
		return nil, err
	}
	return &apiv1.Choice{
		Id:         logicalName,
		Name:       logicalName,
		Label:      logicalName,
		SchemaName: schemaName,
		PgTypeName: pgTypeName,
		Values:     values,
	}, nil
}

func (s *LowcodeService) CreateChoice(ctx context.Context, req *apiv1.CreateChoiceRequest) (*apiv1.CreateChoiceResponse, error) {
	tid, err := s.tenantID(ctx)
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

	data, err := s.tenants.DataPool(ctx)
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
	return &apiv1.CreateChoiceResponse{Choice: ch}, nil
}

func (s *LowcodeService) ListChoices(ctx context.Context, _ *apiv1.ListChoicesRequest) (*apiv1.ListChoicesResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	enums, err := s.listTenantChoiceEnums(ctx, data, tid)
	if err != nil {
		return nil, err
	}
	var resp apiv1.ListChoicesResponse
	for _, e := range enums {
		ch, err := s.choiceToAPI(ctx, tid, e.Schema, e.TypeName)
		if err != nil {
			return nil, err
		}
		resp.Choices = append(resp.Choices, ch)
	}
	return &resp, nil
}

func (s *LowcodeService) GetChoice(ctx context.Context, req *apiv1.GetChoiceRequest) (*apiv1.GetChoiceResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	schema, pgType, err := s.resolveChoicePgTypeRef(ctx, tid, req.Id)
	if err != nil {
		return nil, err
	}
	ch, err := s.choiceToAPI(ctx, tid, schema, pgType)
	if err != nil {
		return nil, err
	}
	return &apiv1.GetChoiceResponse{Choice: ch}, nil
}

func (s *LowcodeService) UpdateChoice(ctx context.Context, req *apiv1.UpdateChoiceRequest) (*apiv1.UpdateChoiceResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	schema, pgType, err := s.resolveChoicePgTypeRef(ctx, tid, req.Id)
	if err != nil {
		return nil, err
	}
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.Values) > 0 {
		literals, err := enumValuesFromItems(req.Values)
		if err != nil {
			return nil, err
		}
		if err := s.addPgEnumValues(ctx, data, schema, pgType, literals); err != nil {
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
	return &apiv1.UpdateChoiceResponse{Choice: ch}, nil
}

func (s *LowcodeService) DeleteChoice(ctx context.Context, req *apiv1.DeleteChoiceRequest) (*apiv1.DeleteChoiceResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	schema, pgType, err := s.resolveChoicePgTypeRef(ctx, tid, req.Id)
	if err != nil {
		return nil, err
	}
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.dropPgEnumType(ctx, data, schema, pgType); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "depends on") {
			return nil, fmt.Errorf("enum type is in use by a column; drop column first")
		}
		return nil, err
	}
	return &apiv1.DeleteChoiceResponse{}, nil
}

// ResolveChoiceValues reads enum labels from PostgreSQL pg_enum.
func (s *LowcodeService) ResolveChoiceValues(ctx context.Context, choiceName string) ([]*apiv1.ChoiceItem, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	schema, pgType, err := s.resolveChoicePgTypeRef(ctx, tid, choiceName)
	if err != nil {
		return nil, err
	}
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	return s.listPgEnumValues(ctx, data, schema, pgType)
}

func (s *LowcodeService) resolveChoicePgType(ctx context.Context, tid, choiceRef string) (schemaName, typeName string, err error) {
	return s.resolveChoicePgTypeRef(ctx, tid, choiceRef)
}
