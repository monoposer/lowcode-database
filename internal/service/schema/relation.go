package schema

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func relationFromLinkColumn(tableID, name, typeID string, cfg map[string]any, createdAt, updatedAt time.Time) *apiv1schema.Relation {
	kind := "MANY_TO_ONE"
	if strings.EqualFold(shared.CfgString(cfg, "cardinality"), "many") || shared.CfgString(cfg, "link_column_id") != "" {
		kind = "ONE_TO_MANY"
	}
	_ = typeID
	return &apiv1schema.Relation{
		Id:             name,
		Name:           name,
		Kind:           kind,
		SourceTableId:  tableID,
		SourceColumnId: name,
		TargetTableId:  shared.CfgString(cfg, "target_table_id"),
		TargetColumnId: shared.CfgString(cfg, "target_column_id"),
		Config:         cfg,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}

func scanLinkAsRelation(rows pgx.Rows) (*apiv1schema.Relation, error) {
	var tableID, name, typeID string
	var cfg map[string]any
	var createdAt, updatedAt time.Time
	if err := rows.Scan(&tableID, &name, &typeID, &cfg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return relationFromLinkColumn(tableID, name, typeID, cfg, createdAt, updatedAt), nil
}

func (s *Schema) CreateRelation(ctx context.Context, req *apiv1schema.CreateRelationRequest) (*apiv1schema.CreateRelationResponse, error) {
	if req.Name == "" || req.SourceTableId == "" || req.TargetTableId == "" {
		return nil, fmt.Errorf("name, source_table_id and target_table_id are required")
	}
	if err := shared.ValidateColumnName(req.Name); err != nil {
		return nil, fmt.Errorf("relation name: %w", err)
	}
	cfg := req.Config
	if cfg == nil {
		cfg = map[string]any{}
	} else {
		cfg = maps.Clone(cfg)
	}
	cfg["target_table_id"] = req.TargetTableId
	if strings.EqualFold(req.Kind, "ONE_TO_MANY") {
		cfg["cardinality"] = "many"
	}

	col, err := s.AddColumn(ctx, &apiv1schema.AddColumnRequest{
		TableId: req.SourceTableId,
		Name:    req.Name,
		TypeId:  "link",
		Config:  cfg,
	})
	if err != nil {
		return nil, err
	}
	c := col.Column
	return &apiv1schema.CreateRelationResponse{
		Relation: relationFromLinkColumn(c.TableId, c.Name, c.TypeId, c.Config, c.CreatedAt, c.UpdatedAt),
	}, nil
}

func (s *Schema) ListRelations(ctx context.Context, req *apiv1schema.ListRelationsRequest) (*apiv1schema.ListRelationsResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	var rows pgx.Rows
	if req != nil && req.TableId != "" {
		tableName, err := s.B.ResolveTableName(ctx, req.TableId)
		if err != nil {
			return nil, err
		}
		rows, err = meta.Query(ctx, `
			SELECT table_id, name, type_id, config, created_at, updated_at
			FROM lc_columns
			WHERE tenant_id = $1 AND base_id = $2
			  AND type_id IN ('link','relationship','relation_fk')
			  AND (table_id = $3 OR config->>'target_table_id' = $3)
			ORDER BY table_id, name`, tid, baseID, tableName)
		if err != nil {
			return nil, err
		}
	} else {
		rows, err = meta.Query(ctx, `
			SELECT table_id, name, type_id, config, created_at, updated_at
			FROM lc_columns
			WHERE tenant_id = $1 AND base_id = $2 AND type_id IN ('link','relationship','relation_fk')
			ORDER BY table_id, name`, tid, baseID)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	var resp apiv1schema.ListRelationsResponse
	for rows.Next() {
		rel, err := scanLinkAsRelation(rows)
		if err != nil {
			return nil, err
		}
		resp.Relations = append(resp.Relations, rel)
	}
	return &resp, rows.Err()
}

func (s *Schema) DeleteRelation(ctx context.Context, req *apiv1schema.DeleteRelationRequest) (*apiv1schema.DeleteRelationResponse, error) {
	if req == nil || req.Name == "" {
		return nil, fmt.Errorf("relation name is required")
	}
	if req.SourceTableId == "" {
		return nil, fmt.Errorf("source_table_id is required")
	}
	_, err := s.DeleteColumn(ctx, &apiv1schema.DeleteColumnRequest{
		TableId: req.SourceTableId,
		Id:      req.Name,
	})
	return &apiv1schema.DeleteRelationResponse{}, err
}
