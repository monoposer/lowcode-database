package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
)

// -------- Relation --------

func (s *LowcodeService) CreateRelation(ctx context.Context, req *apiv1.CreateRelationRequest) (*apiv1.CreateRelationResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" || req.SourceTableId == "" || req.TargetTableId == "" {
		return nil, fmt.Errorf("name, source_table_id and target_table_id are required")
	}
	kind := req.Kind
	if kind == "" {
		kind = "MANY_TO_ONE"
	}
	cfg := req.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	if _, err := s.resolveTableName(ctx, req.SourceTableId); err != nil {
		return nil, fmt.Errorf("source table: %w", err)
	}
	if _, err := s.resolveTableName(ctx, req.TargetTableId); err != nil {
		return nil, fmt.Errorf("target table: %w", err)
	}

	var srcCol, tgtCol *string
	if req.SourceColumnId != "" {
		srcCol = &req.SourceColumnId
	}
	if req.TargetColumnId != "" {
		tgtCol = &req.TargetColumnId
	}

	meta := s.tenants.MetaPool()
	const ins = `
		INSERT INTO lc_relations (tenant_id, name, kind, source_table_id, source_column_id, target_table_id, target_column_id, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, name, kind, source_table_id, source_column_id, target_table_id, target_column_id, config, created_at, updated_at
	`
	row := meta.QueryRow(ctx, ins, tid, req.Name, kind, req.SourceTableId, srcCol, req.TargetTableId, tgtCol, cfg)
	rel, err := scanRelationRow(row)
	if err != nil {
		return nil, err
	}
	return &apiv1.CreateRelationResponse{Relation: rel}, nil
}

func (s *LowcodeService) ListRelations(ctx context.Context, req *apiv1.ListRelationsRequest) (*apiv1.ListRelationsResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	var rows pgx.Rows
	if req.TableId != "" {
		tableName, err := s.resolveTableName(ctx, req.TableId)
		if err != nil {
			return nil, err
		}
		rows, err = meta.Query(ctx, `
			SELECT id, name, kind, source_table_id, source_column_id, target_table_id, target_column_id, config, created_at, updated_at
			FROM lc_relations
			WHERE tenant_id = $1 AND (source_table_id = $2 OR target_table_id = $2)
			ORDER BY name`, tid, tableName)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		rows, err = meta.Query(ctx, `
			SELECT id, name, kind, source_table_id, source_column_id, target_table_id, target_column_id, config, created_at, updated_at
			FROM lc_relations WHERE tenant_id = $1 ORDER BY name`, tid)
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

func (s *LowcodeService) DeleteRelation(ctx context.Context, req *apiv1.DeleteRelationRequest) (*apiv1.DeleteRelationResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	_, err = s.tenants.MetaPool().Exec(ctx, `DELETE FROM lc_relations WHERE id = $1 AND tenant_id = $2`, req.Id, tid)
	return &apiv1.DeleteRelationResponse{}, err
}

func scanRelation(rows pgx.Rows) (*apiv1.Relation, error) {
	var rel apiv1.Relation
	var srcCol, tgtCol *string
	var cfg map[string]any
	var createdAt, updatedAt time.Time
	if err := rows.Scan(&rel.Id, &rel.Name, &rel.Kind, &rel.SourceTableId, &srcCol, &rel.TargetTableId, &tgtCol, &cfg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
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
	if err := row.Scan(&rel.Id, &rel.Name, &rel.Kind, &rel.SourceTableId, &srcCol, &rel.TargetTableId, &tgtCol, &cfg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
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
