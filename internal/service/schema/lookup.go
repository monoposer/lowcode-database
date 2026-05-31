package schema

import (
	"github.com/solat/lowcode-database/internal/service/shared"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/columntype"
)

// ValidateLookupColumnConfig ensures lookup points at a same-table relationship (cardinality one)
// and a physical column on the related table. Config column refs must be logical names.
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
	if shared.EffectiveRelationshipCardinality(norm, shared.CfgString(norm, "link_column_id"), shared.CfgString(norm, "target_column_id")) != "one" {
		return fmt.Errorf("lookup only supports relationship columns with cardinality one (target_column_id link)")
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
			return fmt.Errorf("lookup target_column_id must be a physical column on the related table %q", resolvedTarget)
		}
		return err
	}
	if columntype.IsVirtual(fieldTypeID) {
		return fmt.Errorf("lookup target_column_id must be a physical column on the related table %q", resolvedTarget)
	}
	return nil
}
