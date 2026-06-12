package schema

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apiv1schema "github.com/solat/lowcode-database/internal/apiv1/schema"
	"github.com/solat/lowcode-database/internal/columntype"
	"github.com/solat/lowcode-database/internal/event"
	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/shared"
	"time"
)

func (s *Schema) AddColumn(ctx context.Context, req *apiv1schema.AddColumnRequest) (*apiv1schema.AddColumnResponse, error) {
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

	prep, err := s.prepareAddColumn(ctx, tid, tableKey, req)
	if err != nil {
		return nil, err
	}

	if !prep.isVirtual {
		if err := s.addPhysicalColumnDDL(ctx, data, tid, schemaName, tableKey, req, prep); err != nil {
			return nil, err
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
		prep.typeID,
		req.IsNullable,
		req.Position,
		prep.cfg,
	)

	var c apiv1schema.Column
	var cfgOut map[string]any
	var createdAt, updatedAt time.Time
	if err := row.Scan(&c.Id, &c.TableId, &c.Name, &c.Label, &c.TypeId, &c.IsNullable, &c.Position, &cfgOut, &createdAt, &updatedAt); err != nil {
		if !prep.isVirtual {
			dropPhysicalColumn(ctx, data, schemaName, tableKey, req.Name)
		}
		return nil, err
	}
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt
	if cfgOut != nil {
		c.Config = cfgOut
	}
	if err := s.EnsureColumnResultType(ctx, tid, tableKey, &c); err != nil {
		return nil, err
	}
	PublicColumn(&c)

	s.B.InvalidateTableMetaCache(ctx, tableKey)
	s.B.EmitEvent(ctx, event.MetadataColumnCreated, tableKey, map[string]any{"column": columnToMap(&c)})
	return &apiv1schema.AddColumnResponse{Column: &c}, nil
}

type addColumnPrepared struct {
	typeID      string
	pgType      string
	kind        string
	typeConfig  map[string]any
	isChoiceCol bool
	choiceRef   string
	isVirtual   bool
	cfg         map[string]any
}

func (s *Schema) prepareAddColumn(ctx context.Context, tid, tableKey string, req *apiv1schema.AddColumnRequest) (*addColumnPrepared, error) {
	colType, resolveErr := columntype.Resolve(req.TypeId)

	cfg := req.Config
	if cfg == nil {
		cfg = map[string]any{}
	}

	out := &addColumnPrepared{cfg: cfg}

	if resolveErr == nil {
		out.typeID = colType.ID
		out.pgType = colType.PgType
		out.kind = colType.Kind
		out.typeConfig = colType.Config
	} else {
		choiceRef, isChoiceCol, err := catalog.New(s.B).ResolveChoiceColumnRef(ctx, tid, req.TypeId, cfg)
		if err != nil {
			return nil, err
		}
		if !isChoiceCol {
			return nil, fmt.Errorf("unknown column type %q", req.TypeId)
		}
		out.isChoiceCol = true
		out.choiceRef = choiceRef
		out.typeID = choiceRef
	}

	switch out.kind {
	case "relationship":
		norm, err := s.NormalizeRelationshipConfig(ctx, tid, tableKey, cfg)
		if err != nil {
			return nil, err
		}
		out.cfg = norm
	case "lookup":
		norm, err := s.NormalizeLookupConfig(ctx, tid, tableKey, cfg)
		if err != nil {
			return nil, err
		}
		if err := s.ValidateLookupColumnConfig(ctx, tid, tableKey, norm); err != nil {
			return nil, err
		}
		out.cfg = norm
	case "rollup":
		norm, err := s.NormalizeRollupConfig(ctx, tid, tableKey, cfg)
		if err != nil {
			return nil, err
		}
		out.cfg = norm
	case "relation_fk":
		norm, err := s.NormalizeRelationFKConfig(ctx, tid, cfg)
		if err != nil {
			return nil, err
		}
		if err := s.ValidateRelationFKConfig(ctx, tid, norm); err != nil {
			return nil, err
		}
		out.cfg = norm
	case "formula":
		expr := shared.FormulaExpression(cfg)
		if expr == "" {
			return nil, fmt.Errorf("formula column requires config.expression")
		}
		if err := s.ValidateFormulaExpression(ctx, tableKey, req.Name, expr); err != nil {
			return nil, err
		}
	}

	out.isVirtual = shared.IsVirtualKind(out.kind)

	var err error
	out.cfg, err = s.ApplyColumnResultType(ctx, tid, tableKey, req.Name, out.typeID, out.kind, out.cfg)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Schema) addPhysicalColumnDDL(
	ctx context.Context,
	data *pgxpool.Pool,
	tid, schemaName, tableKey string,
	req *apiv1schema.AddColumnRequest,
	prep *addColumnPrepared,
) error {
	nullSQL := "NULL"
	if !req.IsNullable {
		nullSQL = "NOT NULL"
	}
	colTypeSQL := shared.EffectivePgType(prep.pgType, prep.typeConfig)
	var err error
	if prep.kind == "relation_fk" {
		colTypeSQL, err = s.B.RelationFKColumnPgType(ctx, tid, prep.cfg)
		if err != nil {
			return err
		}
	}
	if prep.isChoiceCol {
		colTypeSQL, err = catalog.New(s.B).ChoiceColumnDDLType(ctx, tid, prep.choiceRef)
		if err != nil {
			return err
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
		return err
	}
	if prep.kind == "relation_fk" {
		if err := s.AddRelationFKConstraint(ctx, data, schemaName, tableKey, req.Name, prep.cfg); err != nil {
			dropPhysicalColumn(ctx, data, schemaName, tableKey, req.Name)
			return err
		}
	}
	return nil
}

func dropPhysicalColumn(ctx context.Context, data *pgxpool.Pool, schemaName, tableKey, colName string) {
	drop := fmt.Sprintf(`ALTER TABLE %s.%s DROP COLUMN IF EXISTS %s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableKey}.Sanitize(),
		pgx.Identifier{colName}.Sanitize())
	_, _ = data.Exec(ctx, drop)
}

func (s *Schema) UpdateColumn(ctx context.Context, req *apiv1schema.UpdateColumnRequest) (*apiv1schema.UpdateColumnResponse, error) {
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

	cfgArg, err := s.normalizeUpdateColumnConfig(ctx, tid, tableKey, curName, curTypeID, req.Config)
	if err != nil {
		return nil, err
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
		if err := s.applyPhysicalColumnChanges(ctx, schemaName, tableKey, curName, newName, curTypeID, newTypeID, curNullable, req.IsNullable); err != nil {
			return nil, err
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
	var c apiv1schema.Column
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
	if err := s.EnsureColumnResultType(ctx, tid, c.TableId, &c); err != nil {
		return nil, err
	}
	PublicColumn(&c)
	s.B.InvalidateTableMetaCache(ctx, c.TableId)
	s.B.EmitEvent(ctx, event.MetadataColumnUpdated, c.TableId, map[string]any{"column": columnToMap(&c)})
	return &apiv1schema.UpdateColumnResponse{Column: &c}, nil
}

func (s *Schema) normalizeUpdateColumnConfig(
	ctx context.Context,
	tid, tableKey, curName, curTypeID string,
	cfg map[string]any,
) (map[string]any, error) {
	if cfg == nil {
		return nil, nil
	}
	cfgArg := cfg
	switch columntype.Kind(curTypeID) {
	case "relationship":
		norm, err := s.NormalizeRelationshipConfig(ctx, tid, tableKey, cfg)
		if err != nil {
			return nil, err
		}
		cfgArg = norm
	case "lookup":
		norm, err := s.NormalizeLookupConfig(ctx, tid, tableKey, cfg)
		if err != nil {
			return nil, err
		}
		if err := s.ValidateLookupColumnConfig(ctx, tid, tableKey, norm); err != nil {
			return nil, err
		}
		cfgArg = norm
	case "formula":
		expr := shared.FormulaExpression(cfg)
		if expr == "" {
			return nil, fmt.Errorf("formula column requires config.expression")
		}
		if err := s.ValidateFormulaExpression(ctx, tableKey, curName, expr); err != nil {
			return nil, err
		}
	case "rollup":
		norm, err := s.NormalizeRollupConfig(ctx, tid, tableKey, cfg)
		if err != nil {
			return nil, err
		}
		cfgArg = norm
	}
	if cfgArg != nil {
		var err error
		cfgArg, err = s.ApplyColumnResultType(ctx, tid, tableKey, curName, curTypeID, columntype.Kind(curTypeID), cfgArg)
		if err != nil {
			return nil, err
		}
	}
	return cfgArg, nil
}

func (s *Schema) applyPhysicalColumnChanges(
	ctx context.Context,
	schemaName, tableKey, curName, newName, curTypeID, newTypeID string,
	curNullable bool,
	reqIsNullable *bool,
) error {
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return err
	}
	if newName != curName {
		if err := shared.ValidateColumnName(newName); err != nil {
			return err
		}
		rename := fmt.Sprintf(`ALTER TABLE %s.%s RENAME COLUMN %s TO %s`,
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{tableKey}.Sanitize(),
			pgx.Identifier{curName}.Sanitize(),
			pgx.Identifier{newName}.Sanitize(),
		)
		if _, err := data.Exec(ctx, rename); err != nil {
			return fmt.Errorf("rename column: %w", err)
		}
	}
	if newTypeID != curTypeID {
		if err := s.AlterColumnType(ctx, schemaName, tableKey, newName, curTypeID, newTypeID); err != nil {
			return err
		}
	}
	if reqIsNullable != nil && *reqIsNullable != curNullable {
		nullSQL := "DROP NOT NULL"
		if !*reqIsNullable {
			nullSQL = "SET NOT NULL"
		}
		alter := fmt.Sprintf(`ALTER TABLE %s.%s ALTER COLUMN %s %s`,
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{tableKey}.Sanitize(),
			pgx.Identifier{newName}.Sanitize(),
			nullSQL,
		)
		if _, err := data.Exec(ctx, alter); err != nil {
			return fmt.Errorf("alter column nullability: %w", err)
		}
	}
	return nil
}
