package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/columntype"
)

// -------- Column --------

func (s *LowcodeService) AddColumn(ctx context.Context, req *apiv1.AddColumnRequest) (*apiv1.AddColumnResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}

	// 允许对外使用 table name 或内部 UUID 作为 table_id，这里解析出逻辑 name 和物理表信息。
	var tableKey, schemaName, tableName string
	if err := meta.QueryRow(ctx, `
		SELECT name, schema_name, table_name
		FROM lc_tables
		WHERE name = $1 AND tenant_id = $2`,
		req.TableId, tid,
	).Scan(&tableKey, &schemaName, &tableName); err != nil {
		return nil, err
	}

	colType, err := columntype.Resolve(req.TypeId)
	if err != nil {
		return nil, err
	}
	typeID := colType.ID
	pgType := colType.PgType
	kind := colType.Kind
	typeConfig := colType.Config

	cfg := req.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	if kind == "relationship" {
		var err error
		cfg, err = NormalizeRelationshipConfig(cfg)
		if err != nil {
			return nil, err
		}
	}
	if kind == "lookup" {
		if err := s.validateLookupColumnConfig(ctx, tid, tableKey, cfg); err != nil {
			return nil, err
		}
	}
	if kind == "rollup" {
		if err := validateRollupConfig(cfg); err != nil {
			return nil, err
		}
	}
	if kind == "relation_fk" {
		if err := s.validateRelationFKConfig(ctx, tid, cfg); err != nil {
			return nil, err
		}
	}
	if kind == "enum" {
		if cfgString(cfg, "choice_id") == "" && cfgString(cfg, "choice_name") == "" {
			return nil, fmt.Errorf("enum column config requires choice_id or choice_name")
		}
	}

	isVirtual := isVirtualKind(kind)

	// 为物理列生成真实 PG 列名；虚拟列则使用一个不会在 SQL 中引用的占位名。
	pgColumn := "c_" + strings.ReplaceAll(uuid.New().String()[:8], "-", "")
	if isVirtual {
		pgColumn = "v_" + strings.ReplaceAll(uuid.New().String()[:8], "-", "")
	} else {
		nullSQL := "NULL"
		if !req.IsNullable {
			nullSQL = "NOT NULL"
		}
		colType := effectivePgType(pgType, typeConfig)
		if kind == "enum" {
			choiceRef := cfgString(cfg, "choice_id")
			if choiceRef == "" {
				choiceRef = cfgString(cfg, "choice_name")
			}
			enumSchema, enumType, err := s.resolveChoicePgType(ctx, tid, choiceRef)
			if err != nil {
				return nil, err
			}
			colType = fmt.Sprintf("%s.%s",
				pgx.Identifier{enumSchema}.Sanitize(),
				pgx.Identifier{enumType}.Sanitize(),
			)
		}
		alter := fmt.Sprintf(`ALTER TABLE %s.%s ADD COLUMN %s %s %s`,
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{tableName}.Sanitize(),
			pgx.Identifier{pgColumn}.Sanitize(),
			colType,
			nullSQL,
		)
		if _, err := data.Exec(ctx, alter); err != nil {
			return nil, err
		}
		if kind == "relation_fk" {
			if err := s.addRelationFKConstraint(ctx, data, schemaName, tableName, pgColumn, cfg); err != nil {
				drop := fmt.Sprintf(`ALTER TABLE %s.%s DROP COLUMN IF EXISTS %s`,
					pgx.Identifier{schemaName}.Sanitize(),
					pgx.Identifier{tableName}.Sanitize(),
					pgx.Identifier{pgColumn}.Sanitize())
				_, _ = data.Exec(ctx, drop)
				return nil, err
			}
		}
	}

	const ins = `
		INSERT INTO lc_columns (tenant_id, table_id, name, type_id, pg_column, is_nullable, position, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, table_id, name, type_id, pg_column, is_nullable, position, config, created_at, updated_at
	`
	row := meta.QueryRow(ctx, ins,
		tid,
		tableKey,
		req.Name,
		typeID,
		pgColumn,
		req.IsNullable,
		req.Position,
		cfg,
	)

	var c apiv1.Column
	var cfgOut map[string]any
	var createdAt, updatedAt time.Time
	if err := row.Scan(&c.Id, &c.TableId, &c.Name, &c.TypeId, &c.PgColumn, &c.IsNullable, &c.Position, &cfgOut, &createdAt, &updatedAt); err != nil {
		if !isVirtual {
			drop := fmt.Sprintf(`ALTER TABLE %s.%s DROP COLUMN IF EXISTS %s`,
				pgx.Identifier{schemaName}.Sanitize(),
				pgx.Identifier{tableName}.Sanitize(),
				pgx.Identifier{pgColumn}.Sanitize())
			_, _ = data.Exec(ctx, drop)
		}
		return nil, err
	}
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt
	if cfgOut != nil {
		c.Config = cfgOut
	}

	s.invalidateTableMetaCache(ctx, tableName)
	return &apiv1.AddColumnResponse{Column: &c}, nil
}

func (s *LowcodeService) ListColumns(ctx context.Context, req *apiv1.ListColumnsRequest) (*apiv1.ListColumnsResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	tableName, err := s.resolveTableName(ctx, req.TableId)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, table_id, name, type_id, pg_column, is_nullable, position, config, created_at, updated_at
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
		if err := rows.Scan(&c.Id, &c.TableId, &c.Name, &c.TypeId, &c.PgColumn, &c.IsNullable, &c.Position, &cfg, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt = createdAt
		c.UpdatedAt = updatedAt
		if cfg != nil {
			c.Config = cfg
		}
		res.Columns = append(res.Columns, &c)
	}
	return &res, rows.Err()
}

func (s *LowcodeService) DeleteColumn(ctx context.Context, req *apiv1.DeleteColumnRequest) (*apiv1.DeleteColumnResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}

	var tableID, schemaName, tableName, pgColumn, typeID string
	if err := meta.QueryRow(ctx, `
		SELECT c.table_id, t.schema_name, t.table_name, c.pg_column, c.type_id
		FROM lc_columns c
		JOIN lc_tables t ON c.table_id = t.name AND c.tenant_id = t.tenant_id
		WHERE c.id = $1 AND c.tenant_id = $2`,
		req.Id, tid,
	).Scan(&tableID, &schemaName, &tableName, &pgColumn, &typeID); err != nil {
		if err == pgx.ErrNoRows {
			return &apiv1.DeleteColumnResponse{}, nil
		}
		return nil, err
	}

	isVirtual := isVirtualKind(columntype.Kind(typeID))
	if !isVirtual {
		drop := fmt.Sprintf(`ALTER TABLE %s.%s DROP COLUMN IF EXISTS %s`,
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{tableName}.Sanitize(),
			pgx.Identifier{pgColumn}.Sanitize())
		if _, err := data.Exec(ctx, drop); err != nil {
			return nil, err
		}
	}

	if _, err := meta.Exec(ctx, `DELETE FROM lc_columns WHERE id = $1 AND tenant_id = $2`, req.Id, tid); err != nil {
		return nil, err
	}

	s.invalidateTableMetaCache(ctx, tableID)
	return &apiv1.DeleteColumnResponse{}, nil
}

// 简化：UpdateColumn 目前只更新元数据，不做 PG 表 rename/alter。
func (s *LowcodeService) UpdateColumn(ctx context.Context, req *apiv1.UpdateColumnRequest) (*apiv1.UpdateColumnResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	cfgArg := req.Config
	if req.Config != nil {
		var typeID, tableKey string
		if err := meta.QueryRow(ctx, `
			SELECT type_id, table_id FROM lc_columns WHERE id = $1 AND tenant_id = $2`,
			req.Id, tid,
		).Scan(&typeID, &tableKey); err != nil {
			return nil, err
		}
		switch columntype.Kind(typeID) {
		case "relationship":
			norm, err := NormalizeRelationshipConfig(req.Config)
			if err != nil {
				return nil, err
			}
			cfgArg = norm
		case "lookup":
			if err := s.validateLookupColumnConfig(ctx, tid, tableKey, req.Config); err != nil {
				return nil, err
			}
		}
	}
	const q = `
		UPDATE lc_columns
		SET name = COALESCE(NULLIF($2, ''), name),
		    is_nullable = COALESCE($3, is_nullable),
		    position = COALESCE(NULLIF($4, 0), position),
		    config = COALESCE($5, config),
		    updated_at = now()
		WHERE id = $1 AND tenant_id = $6
		RETURNING id, table_id, name, type_id, pg_column, is_nullable, position, config, created_at, updated_at
	`
	var c apiv1.Column
	var cfgMap map[string]any
	row := meta.QueryRow(ctx, q, req.Id, req.Name, req.IsNullable, req.Position, cfgArg, tid)
	var createdAt, updatedAt time.Time
	if err := row.Scan(&c.Id, &c.TableId, &c.Name, &c.TypeId, &c.PgColumn, &c.IsNullable, &c.Position, &cfgMap, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt
	if cfgMap != nil {
		c.Config = cfgMap
	}
	s.invalidateTableMetaCache(ctx, c.TableId)
	return &apiv1.UpdateColumnResponse{Column: &c}, nil
}
