package schema

import (
	"context"
	"fmt"

	"github.com/solat/lowcode-database/internal/service/shared"
)

func (s *Schema) LoadRelationshipColumns(ctx context.Context, tableID string, columnIDs []string) ([]shared.RelationshipColumn, error) {
	if len(columnIDs) == 0 {
		return nil, nil
	}
	resolvedName, err := s.B.ResolveTableName(ctx, tableID)
	if err != nil {
		return nil, err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	placeholders := make([]string, len(columnIDs))
	args := make([]any, 0, 2+len(columnIDs))
	args = append(args, resolvedName, tid)
	resolvedNames := make([]string, 0, len(columnIDs))
	for _, ref := range columnIDs {
		name, err := s.ResolveColumnName(ctx, tid, resolvedName, ref)
		if err != nil {
			return nil, err
		}
		resolvedNames = append(resolvedNames, name)
	}
	for i := range resolvedNames {
		placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
		args = append(args, resolvedNames[i])
	}
	q := fmt.Sprintf(`
		SELECT c.name, c.config
		FROM lc_columns c
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.type_id = 'relationship' AND c.name IN (%s)
	`, joinPlaceholders(placeholders))
	rows, err := meta.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []shared.RelationshipColumn
	for rows.Next() {
		var name string
		var cfg map[string]any
		if err := rows.Scan(&name, &cfg); err != nil {
			return nil, err
		}
		rc := shared.RelationshipColumn{Id: name}
		if cfg != nil {
			if v, _ := cfg["target_table_id"].(string); v != "" {
				rc.TargetTableId = v
			}
			if v, _ := cfg["link_column_id"].(string); v != "" {
				rc.LinkColumnId = v
			}
			if v, _ := cfg["target_column_id"].(string); v != "" {
				rc.TargetColumnId = v
			}
		}
		if rc.TargetTableId == "" {
			continue
		}
		if rc.LinkColumnId == "" && rc.TargetColumnId == "" {
			continue
		}
		rc.Cardinality = shared.EffectiveRelationshipCardinality(cfg, rc.LinkColumnId, rc.TargetColumnId)
		out = append(out, rc)
	}
	return out, rows.Err()
}

func joinPlaceholders(parts []string) string {
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += ", " + parts[i]
	}
	return out
}
