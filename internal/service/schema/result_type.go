package schema

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/columntype"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// EnsureColumnResultType fills config.result_type_id and Column.ResultTypeId for API responses.
func (s *Schema) EnsureColumnResultType(ctx context.Context, tenantID, tableKey string, c *apiv1.Column) error {
	if c == nil {
		return nil
	}
	rt, err := s.ResolveColumnResultTypeID(ctx, tenantID, tableKey, c.Name, c.TypeId, c.Config)
	if err != nil {
		return err
	}
	if c.Config == nil {
		c.Config = map[string]any{}
	}
	shared.SetConfigResultTypeID(c.Config, rt)
	c.ResultTypeId = rt
	return nil
}

// ApplyColumnResultType computes result_type_id and writes it into cfg before persist.
func (s *Schema) ApplyColumnResultType(ctx context.Context, tenantID, tableKey, colName, typeID, kind string, cfg map[string]any) (map[string]any, error) {
	if cfg == nil {
		cfg = map[string]any{}
	}
	if override := shared.ConfigResultTypeID(cfg); override != "" {
		if err := shared.ValidateResultTypeID(override); err != nil {
			return nil, fmt.Errorf("config.%s: %w", shared.ConfigKeyResultTypeID, err)
		}
		if kind != "lookup" {
			return cfg, nil
		}
	}
	rt, err := s.ResolveColumnResultTypeID(ctx, tenantID, tableKey, colName, typeID, cfg)
	if err != nil {
		return nil, err
	}
	out := cfg
	shared.SetConfigResultTypeID(out, rt)
	return out, nil
}

// ResolveColumnResultTypeID returns the effective value type for filters, sort, and cells.
func (s *Schema) ResolveColumnResultTypeID(ctx context.Context, tenantID, tableKey, colName, typeID string, cfg map[string]any) (string, error) {
	kind := columntype.Kind(typeID)
	if kind == "" && columntype.IsBuiltIn(typeID) {
		return typeID, nil
	}
	if override := shared.ConfigResultTypeID(cfg); override != "" && kind != "lookup" {
		return override, nil
	}
	visiting := map[string]bool{}
	return s.resolveColumnResultTypeID(ctx, tenantID, tableKey, colName, typeID, cfg, visiting)
}

func (s *Schema) resolveColumnResultTypeID(
	ctx context.Context,
	tenantID, tableKey, colName, typeID string,
	cfg map[string]any,
	visiting map[string]bool,
) (string, error) {
	key := tableKey + ":" + colName
	if visiting[key] {
		return "", fmt.Errorf("result type cycle at %q on table %q", colName, tableKey)
	}
	visiting[key] = true
	defer delete(visiting, key)

	kind := columntype.Kind(typeID)
	switch kind {
	case "formula":
		if override := shared.ConfigResultTypeID(cfg); override != "" {
			return override, nil
		}
		return shared.InferFormulaResultTypeId(shared.FormulaExpression(cfg)), nil
	case "lookup":
		return s.lookupResultTypeID(ctx, tenantID, tableKey, cfg, visiting)
	case "rollup":
		if override := shared.ConfigResultTypeID(cfg); override != "" {
			return override, nil
		}
		return s.rollupResultTypeID(ctx, tenantID, tableKey, cfg, visiting)
	case "relationship":
		return "json", nil
	default:
		if columntype.IsBuiltIn(typeID) {
			return typeID, nil
		}
		// PG ENUM / choice columns use type_id = choice name; treat as text for filters.
		return "text", nil
	}
}

func (s *Schema) lookupResultTypeID(ctx context.Context, tenantID, hostTable string, cfg map[string]any, visiting map[string]bool) (string, error) {
	relName := shared.CfgString(cfg, "relation_column_id")
	fieldName := shared.CfgString(cfg, "target_column_id")
	if relName == "" || fieldName == "" {
		return "text", nil
	}
	meta := s.B.Tenants.MetaPool()
	var relCfg map[string]any
	err := meta.QueryRow(ctx, `
		SELECT config FROM lc_columns
		WHERE tenant_id = $1 AND table_id = $2 AND name = $3 AND type_id = 'relationship'`,
		tenantID, hostTable, relName,
	).Scan(&relCfg)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "text", nil
		}
		return "", err
	}
	normRel, err := shared.NormalizeRelationshipConfig(relCfg)
	if err != nil {
		return "text", nil
	}
	card := shared.EffectiveRelationshipCardinality(normRel, shared.CfgString(normRel, "link_column_id"), shared.CfgString(normRel, "target_column_id"))
	targetTable, err := s.B.ResolveTableName(ctx, shared.CfgString(normRel, "target_table_id"))
	if err != nil {
		return "", err
	}
	targetTypeID, targetCfg, err := s.loadColumnMeta(ctx, tenantID, targetTable, fieldName)
	if err != nil {
		return "", err
	}
	targetRT, err := s.resolveColumnResultTypeID(ctx, tenantID, targetTable, fieldName, targetTypeID, targetCfg, visiting)
	if err != nil {
		return "", err
	}
	if card == "many" {
		return shared.ScalarResultTypeToArray(targetRT), nil
	}
	return targetRT, nil
}

func (s *Schema) loadColumnMeta(ctx context.Context, tenantID, tableKey, colName string) (typeID string, cfg map[string]any, err error) {
	err = s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT type_id, config FROM lc_columns
		WHERE tenant_id = $1 AND table_id = $2 AND name = $3`,
		tenantID, tableKey, colName,
	).Scan(&typeID, &cfg)
	return typeID, cfg, err
}

func (s *Schema) rollupResultTypeID(ctx context.Context, tenantID, hostTable string, cfg map[string]any, visiting map[string]bool) (string, error) {
	agg := shared.CfgString(cfg, "aggregate")
	if agg == "" {
		return "number", nil
	}
	fieldName := shared.CfgString(cfg, "target_column_id")
	if fieldName == "" {
		return shared.RollupResultTypeId(agg, "number"), nil
	}
	relName := shared.CfgString(cfg, "relation_column_id")
	meta := s.B.Tenants.MetaPool()
	var relCfg map[string]any
	if err := meta.QueryRow(ctx, `
		SELECT config FROM lc_columns
		WHERE tenant_id = $1 AND table_id = $2 AND name = $3 AND type_id = 'relationship'`,
		tenantID, hostTable, relName,
	).Scan(&relCfg); err != nil {
		return shared.RollupResultTypeId(agg, "number"), nil
	}
	targetTable, err := s.B.ResolveTableName(ctx, shared.CfgString(relCfg, "target_table_id"))
	if err != nil {
		return "", err
	}
	targetTypeID, targetCfg, err := s.loadColumnMeta(ctx, tenantID, targetTable, fieldName)
	if err != nil {
		return shared.RollupResultTypeId(agg, "number"), nil
	}
	targetRT, err := s.resolveColumnResultTypeID(ctx, tenantID, targetTable, fieldName, targetTypeID, targetCfg, visiting)
	if err != nil {
		return "", err
	}
	return shared.RollupResultTypeId(agg, targetRT), nil
}
