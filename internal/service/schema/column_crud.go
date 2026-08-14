package schema

import (
	"context"
	"fmt"
	"time"
	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/event"
)

func (s *Schema) ListColumns(ctx context.Context, tableName string) ([]*Column, error) {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	resolved, err := s.B.ResolveTableName(ctx, tableName)
	if err != nil {
		return nil, err
	}
	tableName = resolved
	const q = `
		SELECT id, table_name, name, label, type_id, is_nullable, position, config, created_at, updated_at
		FROM lc_columns
		WHERE table_name = $1 AND tenant_id = $2 AND base_id = $3
		ORDER BY position
	`
	rows, err := meta.Query(ctx, q, tableName, tenantID, baseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Column
	for rows.Next() {
		var c Column
		var cfg map[string]any
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&c.Id, &c.TableName, &c.Name, &c.Label, &c.TypeId, &c.IsNullable, &c.Position, &cfg, &createdAt, &updatedAt); err != nil {
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
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (s *Schema) DeleteColumn(ctx context.Context, tableName, id string) error {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return err
	}
	colDBID, err := s.ResolveColumnDBID(ctx, tenantID, tableName, id)
	if err != nil {
		return err
	}
	meta := s.B.Tenants.MetaPool()

	var tableKey string
	if err := meta.QueryRow(ctx, `
		SELECT c.table_name
		FROM lc_columns c
		WHERE c.id = $1 AND c.tenant_id = $2 AND c.base_id = $3`,
		colDBID, tenantID, baseID,
	).Scan(&tableKey); err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	if _, err := meta.Exec(ctx, `
		DELETE FROM lc_columns WHERE id = $1 AND tenant_id = $2 AND base_id = $3`,
		colDBID, tenantID, baseID); err != nil {
		return err
	}

	s.B.InvalidateTableMetaCache(ctx, tableKey)
	s.B.EmitEvent(ctx, event.MetadataColumnDeleted, tableKey, map[string]any{
		"tableName": tableKey, "columnId": colDBID,
	})
	return nil
}

func (s *Schema) AlterColumnType(ctx context.Context, schemaName, tableName, colName, fromTypeID, toTypeID string) error {
	return fmt.Errorf("alter column type is not supported for virtual_records storage")
}
