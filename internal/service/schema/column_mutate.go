package schema

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/event"
	formulacompile "github.com/monoposer/lowcode-database/internal/formula"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"github.com/monoposer/lowcode-database/pkg/typespec"
)

func (s *Schema) AddColumn(ctx context.Context, req *apiv1schema.AddColumnRequest) (*apiv1schema.AddColumnResponse, error) {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	if err := shared.ValidateColumnName(req.Name); err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()

	var tableKey string
	if err := meta.QueryRow(ctx, `
		SELECT name
		FROM lc_tables
		WHERE name = $1 AND tenant_id = $2 AND base_id = $3`,
		req.TableId, tenantID, baseID,
	).Scan(&tableKey); err != nil {
		return nil, err
	}

	prep, err := s.prepareAddColumn(ctx, tenantID, tableKey, req)
	if err != nil {
		return nil, err
	}

	const ins = `
		INSERT INTO lc_columns (tenant_id, base_id, table_id, name, label, type_id, is_nullable, position, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, table_id, name, label, type_id, is_nullable, position, config, created_at, updated_at
	`
	row := meta.QueryRow(ctx, ins,
		tenantID,
		baseID,
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
		return nil, err
	}
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt
	if cfgOut != nil {
		c.Config = cfgOut
	}
	if err := s.EnsureColumnResultType(ctx, tenantID, tableKey, &c); err != nil {
		return nil, err
	}
	PublicColumn(&c)

	s.B.InvalidateTableMetaCache(ctx, tableKey)
	s.B.EmitEvent(ctx, event.MetadataColumnCreated, tableKey, map[string]any{"column": columnToMap(&c)})
	return &apiv1schema.AddColumnResponse{Column: &c}, nil
}

type addColumnPrepared struct {
	typeID          string
	pgType          string
	kind            string
	typeConfig      map[string]any
	isColumnTypeCol bool
	columnTypeRef   string
	isVirtual       bool
	cfg             map[string]any
}

func (s *Schema) prepareAddColumn(ctx context.Context, tid, tableKey string, req *apiv1schema.AddColumnRequest) (*addColumnPrepared, error) {
	colType, resolveErr := columntype.Resolve(req.TypeId)

	cfg := req.Config
	if cfg == nil {
		cfg = map[string]any{}
	}

	out := &addColumnPrepared{cfg: cfg}

	if resolveErr == nil {
		out.typeID = typespec.CanonicalID(req.TypeId)
		out.pgType = colType.PgType
		out.kind = colType.Kind
		out.typeConfig = colType.Config
		if typespec.IsArrayType(req.TypeId) || shared.CfgBool(cfg, "array") {
			if !typespec.AllowsArray(out.typeID) {
				return nil, fmt.Errorf("type %q does not support array", out.typeID)
			}
			cfg["array"] = true
			out.cfg = cfg
			if !strings.HasSuffix(out.pgType, "[]") && out.pgType != "" {
				out.pgType = out.pgType + "[]"
			}
		}
	} else {
		cat := catalog.New(s.B)
		columnTypeRef, isColumnTypeCol, err := cat.ResolveColumnTypeRef(ctx, tid, req.TypeId)
		if err != nil {
			return nil, err
		}
		if !isColumnTypeCol {
			return nil, fmt.Errorf("unknown column type %q", req.TypeId)
		}
		out.isColumnTypeCol = true
		out.columnTypeRef = columnTypeRef
		out.typeID = columnTypeRef
	}

	if out.kind == "link" || typespec.IsLinkType(out.typeID) {
		out.typeID = "link"
		out.kind = "link"
	}

	switch out.kind {
	case "link":
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
	case "formula":
		expr := shared.FormulaExpression(cfg)
		if expr == "" {
			return nil, fmt.Errorf("formula column requires config.expression")
		}
		if err := s.ValidateFormulaExpression(ctx, tableKey, req.Name, expr); err != nil {
			return nil, err
		}
		if _, ok := cfg["deps"]; !ok {
			refs := formulacompile.Refs(expr)
			if len(refs) > 0 {
				cfg["deps"] = refs
				out.cfg = cfg
			}
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
	return fmt.Errorf("physical column DDL is not supported for virtual_records storage")
}

func dropPhysicalColumn(ctx context.Context, data *pgxpool.Pool, schemaName, tableKey, colName string) {
	_ = ctx
	_ = data
	_ = schemaName
	_ = tableKey
	_ = colName
}

func (s *Schema) UpdateColumn(ctx context.Context, req *apiv1schema.UpdateColumnRequest) (*apiv1schema.UpdateColumnResponse, error) {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()

	colDBID, err := s.ResolveColumnDBID(ctx, tenantID, req.TableId, req.Id)
	if err != nil {
		return nil, err
	}

	var curTypeID, tableKey, curName string
	err = meta.QueryRow(ctx, `
		SELECT c.type_id, c.table_id, c.name
		FROM lc_columns c
		WHERE c.id = $1 AND c.tenant_id = $2 AND c.base_id = $3`,
		colDBID, tenantID, baseID,
	).Scan(&curTypeID, &tableKey, &curName)
	if err != nil {
		return nil, err
	}

	cfgArg, err := s.normalizeUpdateColumnConfig(ctx, tenantID, tableKey, curName, curTypeID, req.Config)
	if err != nil {
		return nil, err
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
		WHERE id = $1 AND tenant_id = $6 AND base_id = $9
		RETURNING id, table_id, name, label, type_id, is_nullable, position, config, created_at, updated_at
	`
	var c apiv1schema.Column
	var cfgMap map[string]any
	row := meta.QueryRow(ctx, q, colDBID, req.Name, req.IsNullable, req.Position, cfgArg, tenantID, req.TypeId, req.Label, baseID)
	var createdAt, updatedAt time.Time
	if err := row.Scan(&c.Id, &c.TableId, &c.Name, &c.Label, &c.TypeId, &c.IsNullable, &c.Position, &cfgMap, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt
	if cfgMap != nil {
		c.Config = cfgMap
	}
	if err := s.EnsureColumnResultType(ctx, tenantID, c.TableId, &c); err != nil {
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
	case "link":
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
	return fmt.Errorf("physical column changes are not supported for virtual_records storage")
}
