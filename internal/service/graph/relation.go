package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/service/schema"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// -------- Relation --------

func (s *Graph) resolveRelationRef(ctx context.Context, sourceTableRef, nameRef string) (sourceTableID, relName string, err error) {
	if nameRef == "" {
		return "", "", fmt.Errorf("relation name is required")
	}
	if sourceTableRef == "" {
		return "", "", fmt.Errorf("source_table_id is required")
	}
	sourceTableID, err = s.B.ResolveTableName(ctx, sourceTableRef)
	if err != nil {
		return "", "", err
	}
	if err := shared.ValidateTableName(nameRef); err != nil {
		return "", "", fmt.Errorf("relation name: %w", err)
	}
	return sourceTableID, nameRef, nil
}

func (s *Graph) CreateRelation(ctx context.Context, req *apiv1.CreateRelationRequest) (*apiv1.CreateRelationResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" || req.SourceTableId == "" || req.TargetTableId == "" {
		return nil, fmt.Errorf("name, source_table_id and target_table_id are required")
	}
	if err := shared.ValidateTableName(req.Name); err != nil {
		return nil, err
	}
	kind := req.Kind
	if kind == "" {
		kind = "MANY_TO_ONE"
	}
	cfg := req.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	sourceTable, err := s.B.ResolveTableName(ctx, req.SourceTableId)
	if err != nil {
		return nil, fmt.Errorf("source table: %w", err)
	}
	targetTable, err := s.B.ResolveTableName(ctx, req.TargetTableId)
	if err != nil {
		return nil, fmt.Errorf("target table: %w", err)
	}

	var srcCol, tgtCol *string
	if req.SourceColumnId != "" {
		name, err := schema.New(s.B).ResolveColumnName(ctx, tid, sourceTable, req.SourceColumnId)
		if err != nil {
			return nil, fmt.Errorf("source_column_id: %w", err)
		}
		srcCol = &name
	}
	if req.TargetColumnId != "" {
		name, err := schema.New(s.B).ResolveColumnName(ctx, tid, targetTable, req.TargetColumnId)
		if err != nil {
			return nil, fmt.Errorf("target_column_id: %w", err)
		}
		tgtCol = &name
	}

	meta := s.B.Tenants.MetaPool()
	const ins = `
		INSERT INTO lc_relations (tenant_id, source_table_id, name, kind, source_column_id, target_table_id, target_column_id, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING source_table_id, name, kind, source_column_id, target_table_id, target_column_id, config, created_at, updated_at
	`
	row := meta.QueryRow(ctx, ins, tid, sourceTable, req.Name, kind, srcCol, targetTable, tgtCol, cfg)
	rel, err := scanRelationRow(row)
	if err != nil {
		return nil, err
	}
	return &apiv1.CreateRelationResponse{Relation: rel}, nil
}

func (s *Graph) ListRelations(ctx context.Context, req *apiv1.ListRelationsRequest) (*apiv1.ListRelationsResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	var rows pgx.Rows
	if req.TableId != "" {
		tableName, err := s.B.ResolveTableName(ctx, req.TableId)
		if err != nil {
			return nil, err
		}
		rows, err = meta.Query(ctx, `
			SELECT source_table_id, name, kind, source_column_id, target_table_id, target_column_id, config, created_at, updated_at
			FROM lc_relations
			WHERE tenant_id = $1 AND (source_table_id = $2 OR target_table_id = $2)
			ORDER BY source_table_id, name`, tid, tableName)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		rows, err = meta.Query(ctx, `
			SELECT source_table_id, name, kind, source_column_id, target_table_id, target_column_id, config, created_at, updated_at
			FROM lc_relations WHERE tenant_id = $1 ORDER BY source_table_id, name`, tid)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	var resp apiv1.ListRelationsResponse
	for rows.Next() {
		rel, err := scanRelation(rows)
		if err != nil {
			return nil, err
		}
		resp.Relations = append(resp.Relations, rel)
	}
	return &resp, rows.Err()
}

func (s *Graph) DeleteRelation(ctx context.Context, req *apiv1.DeleteRelationRequest) (*apiv1.DeleteRelationResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	sourceTable, relName, err := s.resolveRelationRef(ctx, req.SourceTableId, req.Name)
	if err != nil {
		return nil, err
	}
	_, err = s.B.Tenants.MetaPool().Exec(ctx, `
		DELETE FROM lc_relations WHERE tenant_id = $1 AND source_table_id = $2 AND name = $3`,
		tid, sourceTable, relName)
	return &apiv1.DeleteRelationResponse{}, err
}

func scanRelation(rows pgx.Rows) (*apiv1.Relation, error) {
	var rel apiv1.Relation
	var srcCol, tgtCol *string
	var cfg map[string]any
	var createdAt, updatedAt time.Time
	if err := rows.Scan(&rel.SourceTableId, &rel.Name, &rel.Kind, &srcCol, &rel.TargetTableId, &tgtCol, &cfg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	rel.Id = rel.Name
	if srcCol != nil {
		rel.SourceColumnId = *srcCol
	}
	if tgtCol != nil {
		rel.TargetColumnId = *tgtCol
	}
	rel.Config = cfg
	rel.CreatedAt = createdAt
	rel.UpdatedAt = updatedAt
	return &rel, nil
}

func scanRelationRow(row pgx.Row) (*apiv1.Relation, error) {
	var rel apiv1.Relation
	var srcCol, tgtCol *string
	var cfg map[string]any
	var createdAt, updatedAt time.Time
	if err := row.Scan(&rel.SourceTableId, &rel.Name, &rel.Kind, &srcCol, &rel.TargetTableId, &tgtCol, &cfg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	rel.Id = rel.Name
	if srcCol != nil {
		rel.SourceColumnId = *srcCol
	}
	if tgtCol != nil {
		rel.TargetColumnId = *tgtCol
	}
	rel.Config = cfg
	rel.CreatedAt = createdAt
	rel.UpdatedAt = updatedAt
	return &rel, nil
}
