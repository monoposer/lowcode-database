package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/columntype"
)

// validateLookupColumnConfig ensures lookup points at a same-table relationship (cardinality one)
// and a physical column on the related table.
func (s *LowcodeService) validateLookupColumnConfig(ctx context.Context, tenantID, tableKey string, cfg map[string]any) error {
	relColID := cfgString(cfg, "relation_column_id")
	fieldColID := cfgString(cfg, "target_column_id")
	if relColID == "" || fieldColID == "" {
		return fmt.Errorf("lookup config requires relation_column_id and target_column_id")
	}
	meta := s.tenants.MetaPool()
	var relCfg map[string]any
	err := meta.QueryRow(ctx, `
		SELECT c.config
		FROM lc_columns c
		WHERE c.id = $1 AND c.tenant_id = $2 AND c.table_id = $3 AND c.type_id = 'relationship'`,
		relColID, tenantID, tableKey,
	).Scan(&relCfg)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("lookup relation_column_id must reference a relationship column on the same table")
	}
	if err != nil {
		return err
	}
	norm, err := NormalizeRelationshipConfig(relCfg)
	if err != nil {
		return fmt.Errorf("lookup: invalid relationship config: %w", err)
	}
	if effectiveRelationshipCardinality(norm, cfgString(norm, "link_column_id"), cfgString(norm, "target_column_id")) != "one" {
		return fmt.Errorf("lookup only supports relationship columns with cardinality one (target_column_id link)")
	}
	targetTable := cfgString(norm, "target_table_id")
	if targetTable == "" {
		return fmt.Errorf("lookup: relationship missing target_table_id")
	}
	resolvedTarget, err := s.resolveTableName(ctx, targetTable)
	if err != nil {
		return fmt.Errorf("lookup: resolve target table: %w", err)
	}
	var fieldTypeID string
	if err := meta.QueryRow(ctx, `
		SELECT type_id FROM lc_columns WHERE id = $1 AND tenant_id = $2 AND table_id = $3`,
		fieldColID, tenantID, resolvedTarget,
	).Scan(&fieldTypeID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("lookup target_column_id must be a physical column on the related table %q", resolvedTarget)
		}
		return err
	}
	if columntype.IsVirtual(fieldTypeID) {
		return fmt.Errorf("lookup target_column_id must be a physical column on the related table %q", resolvedTarget)
	}
	return nil
}
