package schema

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/columntype"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// ValidateLookupColumnConfig ensures lookup points at a same-table relationship and a supported target column.
func (s *Schema) ValidateLookupColumnConfig(ctx context.Context, tenantID, tableKey string, cfg map[string]any) error {
	relColName := shared.CfgString(cfg, "relation_column_id")
	fieldColName := shared.CfgString(cfg, "target_column_id")
	if relColName == "" || fieldColName == "" {
		return fmt.Errorf("lookup config requires relation_column_id and target_column_id")
	}
	meta := s.B.Tenants.MetaPool()
	var relCfg map[string]any
	err := meta.QueryRow(ctx, `
		SELECT c.config
		FROM lc_columns c
		WHERE c.name = $1 AND c.tenant_id = $2 AND c.table_id = $3 AND c.type_id = 'relationship'`,
		relColName, tenantID, tableKey,
	).Scan(&relCfg)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("lookup relation_column_id must reference a relationship column on the same table")
	}
	if err != nil {
		return err
	}
	norm, err := shared.NormalizeRelationshipConfig(relCfg)
	if err != nil {
		return fmt.Errorf("lookup: invalid relationship config: %w", err)
	}
	card := shared.EffectiveRelationshipCardinality(norm, shared.CfgString(norm, "link_column_id"), shared.CfgString(norm, "target_column_id"))
	if card != "one" && card != "many" {
		return fmt.Errorf("lookup requires relationship with cardinality one or many")
	}
	if card == "one" && shared.CfgString(norm, "target_column_id") == "" {
		return fmt.Errorf("lookup with cardinality one requires relationship target_column_id")
	}
	if card == "many" && shared.CfgString(norm, "link_column_id") == "" {
		return fmt.Errorf("lookup with cardinality many requires relationship link_column_id")
	}
	targetTable := shared.CfgString(norm, "target_table_id")
	if targetTable == "" {
		return fmt.Errorf("lookup: relationship missing target_table_id")
	}
	resolvedTarget, err := s.B.ResolveTableName(ctx, targetTable)
	if err != nil {
		return fmt.Errorf("lookup: resolve target table: %w", err)
	}
	var fieldTypeID string
	if err := meta.QueryRow(ctx, `
		SELECT type_id FROM lc_columns WHERE name = $1 AND tenant_id = $2 AND table_id = $3`,
		fieldColName, tenantID, resolvedTarget,
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

	meta := s.B.Tenants.MetaPool()
	var typeID string
	var cfg map[string]any
	err := meta.QueryRow(ctx, `
		SELECT type_id, config FROM lc_columns
		WHERE name = $1 AND tenant_id = $2 AND table_id = $3`,
		colName, tenantID, tableKey,
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
		WHERE name = $1 AND tenant_id = $2 AND table_id = $3 AND type_id = 'relationship'`,
		relName, tenantID, tableKey,
	).Scan(&relCfg); err != nil {
		return err
	}
	targetTable, err := s.B.ResolveTableName(ctx, shared.CfgString(relCfg, "target_table_id"))
	if err != nil {
		return err
	}
	return s.validateLookupTargetChain(ctx, tenantID, targetTable, targetCol, stack)
}
