package schema

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"
	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func relationFromLinkColumn(tableName, name, typeID string, cfg map[string]any, createdAt, updatedAt time.Time) *Relation {
	kind := "MANY_TO_ONE"
	if strings.EqualFold(shared.CfgString(cfg, "cardinality"), "many") || shared.CfgString(cfg, "link_column_id") != "" {
		kind = "ONE_TO_MANY"
	}
	_ = typeID
	return &Relation{
		Id:             name,
		Name:           name,
		Kind:           kind,
		SourceTableName:  tableName,
		SourceColumnId: name,
		TargetTableName:  shared.CfgString(cfg, "target_table_name"),
		TargetColumnId: shared.CfgString(cfg, "target_column_id"),
		Config:         cfg,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}

func scanLinkAsRelation(rows pgx.Rows) (*Relation, error) {
	var tableName, name, typeID string
	var cfg map[string]any
	var createdAt, updatedAt time.Time
	if err := rows.Scan(&tableName, &name, &typeID, &cfg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return relationFromLinkColumn(tableName, name, typeID, cfg, createdAt, updatedAt), nil
}

func (s *Schema) CreateRelation(ctx context.Context, req *Relation) (*Relation, error) {
	if req.Name == "" || req.SourceTableName == "" || req.TargetTableName == "" {
		return nil, fmt.Errorf("name, source_table_name and target_table_name are required")
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
	cfg["target_table_name"] = req.TargetTableName
	if strings.EqualFold(req.Kind, "ONE_TO_MANY") {
		cfg["cardinality"] = "many"
	}

	col, err := s.AddColumn(ctx, &Column{
		TableName: req.SourceTableName,
		Name:    req.Name,
		TypeId:  "link",
		Config:  cfg,
	})
	if err != nil {
		return nil, err
	}
	return relationFromLinkColumn(col.TableName, col.Name, col.TypeId, col.Config, col.CreatedAt, col.UpdatedAt), nil
}

func (s *Schema) ListRelations(ctx context.Context, tableName string) ([]*Relation, error) {
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
	if tableName != "" {
		tableName, err := s.B.ResolveTableName(ctx, tableName)
		if err != nil {
			return nil, err
		}
		rows, err = meta.Query(ctx, `
			SELECT table_name, name, type_id, config, created_at, updated_at
			FROM lc_columns
			WHERE tenant_id = $1 AND base_id = $2
			  AND type_id IN ('link')
			  AND (table_name = $3 OR config->>'target_table_name' = $3)
			ORDER BY table_name, name`, tid, baseID, tableName)
		if err != nil {
			return nil, err
		}
	} else {
		rows, err = meta.Query(ctx, `
			SELECT table_name, name, type_id, config, created_at, updated_at
			FROM lc_columns
			WHERE tenant_id = $1 AND base_id = $2 AND type_id IN ('link')
			ORDER BY table_name, name`, tid, baseID)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	var out []*Relation
	for rows.Next() {
		rel, err := scanLinkAsRelation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	return out, rows.Err()
}

func (s *Schema) DeleteRelation(ctx context.Context, sourceTableName, name string) error {
	if name == "" {
		return fmt.Errorf("relation name is required")
	}
	if sourceTableName == "" {
		return fmt.Errorf("source_table_name is required")
	}
	return s.DeleteColumn(ctx, sourceTableName, name)
}
