package schema

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"time"
)

func (s *Schema) ListColumns(ctx context.Context, req *apiv1schema.ListColumnsRequest) (*apiv1schema.ListColumnsResponse, error) {
	tid, err := s.B.TenantID(ctx)
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
		WHERE table_id = $1 AND tenant_id = $2
		ORDER BY position
	`
	rows, err := meta.Query(ctx, q, tableName, tid)
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
		if err := s.EnsureColumnResultType(ctx, tid, tableName, &c); err != nil {
			return nil, err
		}
		PublicColumn(&c)
		res.Columns = append(res.Columns, &c)
	}
	return &res, rows.Err()
}

func (s *Schema) DeleteColumn(ctx context.Context, req *apiv1schema.DeleteColumnRequest) (*apiv1schema.DeleteColumnResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	colDBID, err := s.ResolveColumnDBID(ctx, tid, req.TableId, req.Id)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}

	var tableID, schemaName, colName, typeID string
	if err := meta.QueryRow(ctx, `
		SELECT c.table_id, t.schema_name, c.name, c.type_id
		FROM lc_columns c
		JOIN lc_tables t ON c.table_id = t.name AND c.tenant_id = t.tenant_id
		WHERE c.id = $1 AND c.tenant_id = $2`,
		colDBID, tid,
	).Scan(&tableID, &schemaName, &colName, &typeID); err != nil {
		if err == pgx.ErrNoRows {
			return &apiv1schema.DeleteColumnResponse{}, nil
		}
		return nil, err
	}

	if !shared.IsVirtualKind(columntype.Kind(typeID)) {
		drop := fmt.Sprintf(`ALTER TABLE %s.%s DROP COLUMN IF EXISTS %s`,
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{tableID}.Sanitize(),
			pgx.Identifier{colName}.Sanitize())
		if _, err := data.Exec(ctx, drop); err != nil {
			return nil, err
		}
	}

	if _, err := meta.Exec(ctx, `DELETE FROM lc_columns WHERE id = $1 AND tenant_id = $2`, colDBID, tid); err != nil {
		return nil, err
	}

	s.B.InvalidateTableMetaCache(ctx, tableID)
	s.B.EmitEvent(ctx, event.MetadataColumnDeleted, tableID, map[string]any{
		"tableId": tableID, "columnId": colDBID,
	})
	return &apiv1schema.DeleteColumnResponse{}, nil
}

func (s *Schema) AlterColumnType(ctx context.Context, schemaName, tableName, colName, fromTypeID, toTypeID string) error {
	if columntype.IsVirtual(fromTypeID) || columntype.IsVirtual(toTypeID) {
		return fmt.Errorf("cannot change type for virtual columns")
	}
	if !columntype.IsBuiltIn(fromTypeID) || !columntype.IsBuiltIn(toTypeID) {
		return fmt.Errorf("column type change not supported for choice columns")
	}
	if columntype.Kind(fromTypeID) == "relation_fk" || columntype.Kind(toTypeID) == "relation_fk" {
		return fmt.Errorf("column type change not supported for relation_fk")
	}
	newColType, err := columntype.Resolve(toTypeID)
	if err != nil {
		return err
	}
	fromColType, err := columntype.Resolve(fromTypeID)
	if err != nil {
		return err
	}
	toPgSQL := shared.EffectivePgType(newColType.PgType, newColType.Config)
	fromPgSQL := shared.EffectivePgType(fromColType.PgType, fromColType.Config)
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return err
	}
	usingExpr := shared.ColumnAlterTypeUsing(fromPgSQL, toPgSQL, colName)
	alter := fmt.Sprintf(`ALTER TABLE %s.%s ALTER COLUMN %s TYPE %s USING %s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		pgx.Identifier{colName}.Sanitize(),
		toPgSQL,
		usingExpr,
	)
	if _, err := data.Exec(ctx, alter); err != nil {
		return fmt.Errorf("alter column type: %w", err)
	}
	return nil
}
