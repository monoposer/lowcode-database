package schema

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// ValidateLookupColumnConfig ensures lookup points at a same-table link column and a supported target column.
func (s *Schema) ValidateLookupColumnConfig(ctx context.Context, tenantID, tableKey string, cfg map[string]any) error {
	relColName := shared.CfgString(cfg, "relation_column_id")
	fieldColName := shared.CfgString(cfg, "target_column_id")
	if relColName == "" || fieldColName == "" {
		return fmt.Errorf("lookup config requires relation_column_id and target_column_id")
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return err
	}
	meta := s.B.Tenants.MetaPool()
	var relCfg map[string]any
	err = meta.QueryRow(ctx, `
		SELECT c.config
		FROM lc_columns c
		WHERE c.name = $1 AND c.tenant_id = $2 AND c.base_id = $3 AND c.table_name = $4 AND c.type_id = 'link'`,
		relColName, tenantID, baseID, tableKey,
	).Scan(&relCfg)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("lookup relation_column_id must reference a link column on the same table")
	}
	if err != nil {
		return err
	}
	norm, err := shared.NormalizeRelationshipConfig(relCfg)
	if err != nil {
		return fmt.Errorf("lookup: invalid link config: %w", err)
	}
	card := shared.EffectiveRelationshipCardinality(norm, shared.CfgString(norm, "link_column_id"), shared.CfgString(norm, "target_column_id"))
	if card != "one" && card != "many" {
		return fmt.Errorf("lookup requires link with cardinality one or many")
	}
	targetTable := shared.CfgString(norm, "target_table_name")
	if targetTable == "" {
		return fmt.Errorf("lookup: link missing target_table_name")
	}
	resolvedTarget, err := s.B.ResolveTableName(ctx, targetTable)
	if err != nil {
		return fmt.Errorf("lookup: resolve target table: %w", err)
	}
	var fieldTypeID string
	if err := meta.QueryRow(ctx, `
		SELECT type_id FROM lc_columns WHERE name = $1 AND tenant_id = $2 AND base_id = $3 AND table_name = $4`,
		fieldColName, tenantID, baseID, resolvedTarget,
	).Scan(&fieldTypeID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("lookup target_column_id %q not found on related table %q", fieldColName, resolvedTarget)
		}
		return err
	}
	kind := columntype.Kind(fieldTypeID)
	if !shared.LookupTargetAllowed(kind) {
		return fmt.Errorf("lookup target_column_id %q (%s) cannot be used as a lookup target", fieldColName, fieldTypeID)
	}
	if kind == "lookup" {
		if err := s.validateLookupTargetChain(ctx, tenantID, resolvedTarget, fieldColName, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *Schema) validateLookupTargetChain(ctx context.Context, tenantID, tableKey, colName string, stack map[string]bool) error {
	if stack == nil {
		stack = make(map[string]bool)
	}
	key := tableKey + ":" + colName
	if stack[key] {
		return fmt.Errorf("lookup target cycle involving %q on table %q", colName, tableKey)
	}
	stack[key] = true
	defer delete(stack, key)

	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return err
	}
	meta := s.B.Tenants.MetaPool()
	var typeID string
	var cfg map[string]any
	err = meta.QueryRow(ctx, `
		SELECT type_id, config FROM lc_columns
		WHERE name = $1 AND tenant_id = $2 AND base_id = $3 AND table_name = $4`,
		colName, tenantID, baseID, tableKey,
	).Scan(&typeID, &cfg)
	if err != nil {
		return err
	}
	if columntype.Kind(typeID) != "lookup" {
		return nil
	}
	relName := shared.CfgString(cfg, "relation_column_id")
	targetCol := shared.CfgString(cfg, "target_column_id")
	if relName == "" || targetCol == "" {
		return fmt.Errorf("lookup %q: incomplete config", colName)
	}
	var relCfg map[string]any
	if err := meta.QueryRow(ctx, `
		SELECT config FROM lc_columns
		WHERE name = $1 AND tenant_id = $2 AND base_id = $3 AND table_name = $4 AND type_id = 'link'`,
		relName, tenantID, baseID, tableKey,
	).Scan(&relCfg); err != nil {
		return err
	}
	targetTable, err := s.B.ResolveTableName(ctx, shared.CfgString(relCfg, "target_table_name"))
	if err != nil {
		return err
	}
	return s.validateLookupTargetChain(ctx, tenantID, targetTable, targetCol, stack)
}
