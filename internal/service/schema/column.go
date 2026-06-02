package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/columntype"
	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// -------- Column --------

func (s *Schema) AddColumn(ctx context.Context, req *apiv1.AddColumnRequest) (*apiv1.AddColumnResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if err := shared.ValidateColumnName(req.Name); err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}

	var tableKey, schemaName string
	if err := meta.QueryRow(ctx, `
		SELECT name, schema_name
		FROM lc_tables
		WHERE name = $1 AND tenant_id = $2`,
		req.TableId, tid,
	).Scan(&tableKey, &schemaName); err != nil {
		return nil, err
	}

	colType, resolveErr := columntype.Resolve(req.TypeId)

	cfg := req.Config
	if cfg == nil {
		cfg = map[string]any{}
	}

	var typeID, pgType, kind string
	var typeConfig map[string]any
	var isChoiceCol bool
	var choiceRef string

	if resolveErr == nil {
		typeID = colType.ID
		pgType = colType.PgType
		kind = colType.Kind
		typeConfig = colType.Config
	} else {
		var err error
		choiceRef, isChoiceCol, err = catalog.New(s.B).ResolveChoiceColumnRef(ctx, tid, req.TypeId, cfg)
		if err != nil {
			return nil, err
		}
		if !isChoiceCol {
			return nil, fmt.Errorf("unknown column type %q", req.TypeId)
		}
		typeID = choiceRef
	}

	if kind == "relationship" {
		var err error
		cfg, err = s.NormalizeRelationshipConfig(ctx, tid, tableKey, cfg)
		if err != nil {
			return nil, err
		}
	}
	if kind == "lookup" {
		var err error
		cfg, err = s.NormalizeLookupConfig(ctx, tid, tableKey, cfg)
		if err != nil {
			return nil, err
		}
		if err := s.ValidateLookupColumnConfig(ctx, tid, tableKey, cfg); err != nil {
			return nil, err
		}
	}
	if kind == "rollup" {
		var err error
		cfg, err = s.NormalizeRollupConfig(ctx, tid, tableKey, cfg)
		if err != nil {
			return nil, err
		}
	}
	if kind == "relation_fk" {
		var err error
		cfg, err = s.NormalizeRelationFKConfig(ctx, tid, cfg)
		if err != nil {
			return nil, err
		}
		if err := s.ValidateRelationFKConfig(ctx, tid, cfg); err != nil {
			return nil, err
		}
	}
	if kind == "formula" {
		expr := shared.FormulaExpression(cfg)
		if expr == "" {
			return nil, fmt.Errorf("formula column requires config.expression")
		}
		if err := s.ValidateFormulaExpression(ctx, tableKey, req.Name, expr); err != nil {
			return nil, err
		}
	}

	isVirtual := shared.IsVirtualKind(kind)

	if !isVirtual {
		nullSQL := "NULL"
		if !req.IsNullable {
			nullSQL = "NOT NULL"
		}
		colTypeSQL := shared.EffectivePgType(pgType, typeConfig)
		if kind == "relation_fk" {
			colTypeSQL, err = s.B.RelationFKColumnPgType(ctx, tid, cfg)
			if err != nil {
				return nil, err
			}
		}
		if isChoiceCol {
			var err error
			colTypeSQL, err = catalog.New(s.B).ChoiceColumnDDLType(ctx, tid, choiceRef)
			if err != nil {
				return nil, err
			}
		}
		alter := fmt.Sprintf(`ALTER TABLE %s.%s ADD COLUMN %s %s %s`,
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{tableKey}.Sanitize(),
			pgx.Identifier{req.Name}.Sanitize(),
			colTypeSQL,
			nullSQL,
		)
		if _, err := data.Exec(ctx, alter); err != nil {
			return nil, err
		}
		if kind == "relation_fk" {
			if err := s.AddRelationFKConstraint(ctx, data, schemaName, tableKey, req.Name, cfg); err != nil {
				drop := fmt.Sprintf(`ALTER TABLE %s.%s DROP COLUMN IF EXISTS %s`,
					pgx.Identifier{schemaName}.Sanitize(),
					pgx.Identifier{tableKey}.Sanitize(),
					pgx.Identifier{req.Name}.Sanitize())
				_, _ = data.Exec(ctx, drop)
				return nil, err
			}
		}
	}

	const ins = `
		INSERT INTO lc_columns (tenant_id, table_id, name, label, type_id, is_nullable, position, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, table_id, name, label, type_id, is_nullable, position, config, created_at, updated_at
	`
	row := meta.QueryRow(ctx, ins,
		tid,
		tableKey,
		req.Name,
		req.Label,
		typeID,
		req.IsNullable,
		req.Position,
		cfg,
	)

	var c apiv1.Column
	var cfgOut map[string]any
	var createdAt, updatedAt time.Time
	if err := row.Scan(&c.Id, &c.TableId, &c.Name, &c.Label, &c.TypeId, &c.IsNullable, &c.Position, &cfgOut, &createdAt, &updatedAt); err != nil {
		if !isVirtual {
			drop := fmt.Sprintf(`ALTER TABLE %s.%s DROP COLUMN IF EXISTS %s`,
				pgx.Identifier{schemaName}.Sanitize(),
				pgx.Identifier{tableKey}.Sanitize(),
				pgx.Identifier{req.Name}.Sanitize())
			_, _ = data.Exec(ctx, drop)
		}
		return nil, err
	}
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt
	if cfgOut != nil {
		c.Config = cfgOut
	}
	PublicColumn(&c)

	s.B.InvalidateTableMetaCache(ctx, tableKey)
	return &apiv1.AddColumnResponse{Column: &c}, nil
}

func (s *Schema) ListColumns(ctx context.Context, req *apiv1.ListColumnsRequest) (*apiv1.ListColumnsResponse, error) {
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

	var res apiv1.ListColumnsResponse
	for rows.Next() {
		var c apiv1.Column
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
		PublicColumn(&c)
		res.Columns = append(res.Columns, &c)
	}
	return &res, rows.Err()
}

func (s *Schema) DeleteColumn(ctx context.Context, req *apiv1.DeleteColumnRequest) (*apiv1.DeleteColumnResponse, error) {
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
			return &apiv1.DeleteColumnResponse{}, nil
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
	return &apiv1.DeleteColumnResponse{}, nil
}

func (s *Schema) UpdateColumn(ctx context.Context, req *apiv1.UpdateColumnRequest) (*apiv1.UpdateColumnResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()

	colDBID, err := s.ResolveColumnDBID(ctx, tid, req.TableId, req.Id)
	if err != nil {
		return nil, err
	}

	var curTypeID, tableKey, curName, schemaName string
	var curNullable bool
	err = meta.QueryRow(ctx, `
		SELECT c.type_id, c.table_id, c.name, c.is_nullable, t.schema_name
		FROM lc_columns c
		JOIN lc_tables t ON c.table_id = t.name AND c.tenant_id = t.tenant_id
		WHERE c.id = $1 AND c.tenant_id = $2`,
		colDBID, tid,
	).Scan(&curTypeID, &tableKey, &curName, &curNullable, &schemaName)
	if err != nil {
		return nil, err
	}

	cfgArg := req.Config
	if req.Config != nil {
		switch columntype.Kind(curTypeID) {
		case "relationship":
			norm, err := s.NormalizeRelationshipConfig(ctx, tid, tableKey, req.Config)
			if err != nil {
				return nil, err
			}
			cfgArg = norm
		case "lookup":
			norm, err := s.NormalizeLookupConfig(ctx, tid, tableKey, req.Config)
			if err != nil {
				return nil, err
			}
			if err := s.ValidateLookupColumnConfig(ctx, tid, tableKey, norm); err != nil {
				return nil, err
			}
			cfgArg = norm
		case "formula":
			expr := shared.FormulaExpression(req.Config)
			if expr == "" {
				return nil, fmt.Errorf("formula column requires config.expression")
			}
			if err := s.ValidateFormulaExpression(ctx, tableKey, curName, expr); err != nil {
				return nil, err
			}
		}
	}

	isVirtual := columntype.IsVirtual(curTypeID)
	newTypeID := curTypeID
	if req.TypeId != "" {
		newTypeID = req.TypeId
	}

	newName := curName
	if req.Name != "" {
		newName = req.Name
	}

	if !isVirtual {
		data, err := s.B.Tenants.DataPool(ctx)
		if err != nil {
			return nil, err
		}
		if newName != curName {
			if err := shared.ValidateColumnName(newName); err != nil {
				return nil, err
			}
			rename := fmt.Sprintf(`ALTER TABLE %s.%s RENAME COLUMN %s TO %s`,
				pgx.Identifier{schemaName}.Sanitize(),
				pgx.Identifier{tableKey}.Sanitize(),
				pgx.Identifier{curName}.Sanitize(),
				pgx.Identifier{newName}.Sanitize(),
			)
			if _, err := data.Exec(ctx, rename); err != nil {
				return nil, fmt.Errorf("rename column: %w", err)
			}
		}
		if newTypeID != curTypeID {
			if err := s.AlterColumnType(ctx, schemaName, tableKey, newName, curTypeID, newTypeID); err != nil {
				return nil, err
			}
		}
		if req.IsNullable != nil && *req.IsNullable != curNullable {
			nullSQL := "DROP NOT NULL"
			if !*req.IsNullable {
				nullSQL = "SET NOT NULL"
			}
			alter := fmt.Sprintf(`ALTER TABLE %s.%s ALTER COLUMN %s %s`,
				pgx.Identifier{schemaName}.Sanitize(),
				pgx.Identifier{tableKey}.Sanitize(),
				pgx.Identifier{newName}.Sanitize(),
				nullSQL,
			)
			if _, err := data.Exec(ctx, alter); err != nil {
				return nil, fmt.Errorf("alter column nullability: %w", err)
			}
		}
	}

	const q = `
		UPDATE lc_columns
		SET name = COALESCE(NULLIF($2, ''), name),
		    label = COALESCE(NULLIF($8, ''), label),
		    type_id = COALESCE(NULLIF($7, ''), type_id),
		    is_nullable = COALESCE($3, is_nullable),
		    position = COALESCE(NULLIF($4, 0), position),
		    config = COALESCE($5, config),
		    updated_at = now()
		WHERE id = $1 AND tenant_id = $6
		RETURNING id, table_id, name, label, type_id, is_nullable, position, config, created_at, updated_at
	`
	var c apiv1.Column
	var cfgMap map[string]any
	row := meta.QueryRow(ctx, q, colDBID, req.Name, req.IsNullable, req.Position, cfgArg, tid, req.TypeId, req.Label)
	var createdAt, updatedAt time.Time
	if err := row.Scan(&c.Id, &c.TableId, &c.Name, &c.Label, &c.TypeId, &c.IsNullable, &c.Position, &cfgMap, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt
	if cfgMap != nil {
		c.Config = cfgMap
	}
	PublicColumn(&c)
	s.B.InvalidateTableMetaCache(ctx, c.TableId)
	return &apiv1.UpdateColumnResponse{Column: &c}, nil
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
