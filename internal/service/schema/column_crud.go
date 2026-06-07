package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/event"
)

func (s *Schema) ListColumns(ctx context.Context, req *apiv1schema.ListColumnsRequest) (*apiv1schema.ListColumnsResponse, error) {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	tableName, err := s.B.ResolveTableName(ctx, req.TableId)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, table_id, name, label, type_id, is_nullable, position, config, created_at, updated_at
		FROM lc_columns
		WHERE table_id = $1 AND tenant_id = $2 AND base_id = $3
		ORDER BY position
	`
	rows, err := meta.Query(ctx, q, tableName, tenantID, baseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res apiv1schema.ListColumnsResponse
	for rows.Next() {
		var c apiv1schema.Column
		var cfg map[string]any
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&c.Id, &c.TableId, &c.Name, &c.Label, &c.TypeId, &c.IsNullable, &c.Position, &cfg, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt = createdAt
		c.UpdatedAt = updatedAt
		if cfg != nil {
			c.Config = cfg
		}
		if err := s.EnsureColumnResultType(ctx, tenantID, tableName, &c); err != nil {
			return nil, err
		}
		PublicColumn(&c)
		res.Columns = append(res.Columns, &c)
	}
	return &res, rows.Err()
}

func (s *Schema) DeleteColumn(ctx context.Context, req *apiv1schema.DeleteColumnRequest) (*apiv1schema.DeleteColumnResponse, error) {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	colDBID, err := s.ResolveColumnDBID(ctx, tenantID, req.TableId, req.Id)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()

	var tableID string
	if err := meta.QueryRow(ctx, `
		SELECT c.table_id
		FROM lc_columns c
		WHERE c.id = $1 AND c.tenant_id = $2 AND c.base_id = $3`,
		colDBID, tenantID, baseID,
	).Scan(&tableID); err != nil {
		if err == pgx.ErrNoRows {
			return &apiv1schema.DeleteColumnResponse{}, nil
		}
		return nil, err
	}

	if _, err := meta.Exec(ctx, `
		DELETE FROM lc_columns WHERE id = $1 AND tenant_id = $2 AND base_id = $3`,
		colDBID, tenantID, baseID); err != nil {
		return nil, err
	}

	s.B.InvalidateTableMetaCache(ctx, tableID)
	s.B.EmitEvent(ctx, event.MetadataColumnDeleted, tableID, map[string]any{
		"tableId": tableID, "columnId": colDBID,
	})
	return &apiv1schema.DeleteColumnResponse{}, nil
}

func (s *Schema) AlterColumnType(ctx context.Context, schemaName, tableName, colName, fromTypeID, toTypeID string) error {
	return fmt.Errorf("alter column type is not supported for virtual_records storage")
}
