package schema

import (
	"context"
	"fmt"

	"github.com/solat/lowcode-database/internal/service/shared"
)

// LoadManyRelationshipColumns returns many-cardinality relationship columns keyed by column name.
func (s *Schema) LoadManyRelationshipColumns(ctx context.Context, tableID string) (map[string]shared.RelationshipColumn, error) {
	resolvedName, err := s.B.ResolveTableName(ctx, tableID)
	if err != nil {
		return nil, err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	const q = `
		SELECT c.name, c.config
		FROM lc_columns c
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.type_id = 'relationship'
	`
	rows, err := meta.Query(ctx, q, resolvedName, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]shared.RelationshipColumn)
	for rows.Next() {
		var name string
		var cfg map[string]any
		if err := rows.Scan(&name, &cfg); err != nil {
			return nil, err
		}
		linkID := shared.CfgString(cfg, "link_column_id")
		targetColID := shared.CfgString(cfg, "target_column_id")
		card := shared.EffectiveRelationshipCardinality(cfg, linkID, targetColID)
		if card != "many" {
			continue
		}
		targetTable := shared.CfgString(cfg, "target_table_id")
		if targetTable == "" || linkID == "" {
			continue
		}
		out[name] = shared.RelationshipColumn{
			Id:            name,
			TargetTableId: targetTable,
			LinkColumnId:  linkID,
			Cardinality:   "many",
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// LoadOneRelationshipColumns returns one-cardinality relationship columns keyed by column name.
func (s *Schema) LoadOneRelationshipColumns(ctx context.Context, tableID string) (map[string]shared.RelationshipColumn, error) {
	resolvedName, err := s.B.ResolveTableName(ctx, tableID)
	if err != nil {
		return nil, err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	const q = `
		SELECT c.name, c.config
		FROM lc_columns c
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.type_id = 'relationship'
	`
	rows, err := meta.Query(ctx, q, resolvedName, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]shared.RelationshipColumn)
	for rows.Next() {
		var name string
		var cfg map[string]any
		if err := rows.Scan(&name, &cfg); err != nil {
			return nil, err
		}
		linkID := shared.CfgString(cfg, "link_column_id")
		targetColID := shared.CfgString(cfg, "target_column_id")
		card := shared.EffectiveRelationshipCardinality(cfg, linkID, targetColID)
		if card != "one" {
			continue
		}
		targetTable := shared.CfgString(cfg, "target_table_id")
		if targetTable == "" || targetColID == "" {
			continue
		}
		out[name] = shared.RelationshipColumn{
			Id:             name,
			TargetTableId:  targetTable,
			TargetColumnId: targetColID,
			Cardinality:    "one",
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// MustFKColumnName resolves target_column_id to physical FK column name on the host table.
func (s *Schema) MustFKColumnName(ctx context.Context, tenantID, hostTableID, fkColumnRef string) (string, error) {
	name, err := s.ResolveColumnName(ctx, tenantID, hostTableID, fkColumnRef)
	if err != nil {
		return "", fmt.Errorf("fk column: %w", err)
	}
	return name, nil
}

// RelationshipColumnNamesSet returns a set of keys for ClassifySaveGraphFields.
func RelationshipColumnNamesSet(rels map[string]shared.RelationshipColumn) map[string]struct{} {
	set := make(map[string]struct{}, len(rels))
	for k := range rels {
		set[k] = struct{}{}
	}
	return set
}

// MustLinkColumnName resolves link_column_id to physical column name on child table.
func (s *Schema) MustLinkColumnName(ctx context.Context, tenantID, childTableID, linkColumnRef string) (string, error) {
	name, err := s.ResolveColumnName(ctx, tenantID, childTableID, linkColumnRef)
	if err != nil {
		return "", fmt.Errorf("link column: %w", err)
	}
	return name, nil
}
