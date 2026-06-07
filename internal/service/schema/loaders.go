package schema

import (
	"context"
	"fmt"
	"strings"

	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/service/shared"
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
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	placeholders := make([]string, len(columnIDs))
	args := make([]any, 0, 3+len(columnIDs))
	args = append(args, resolvedName, tid, baseID)
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
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.base_id = $3 AND c.type_id IN ('link','relationship','relation_fk') AND c.name IN (%s)
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
			if rc.TargetTableId == "" {
				if v, _ := cfg["to_table_id"].(string); v != "" {
					rc.TargetTableId = v
				}
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

// -------- ER Diagram --------

func (s *Schema) GetERDiagram(ctx context.Context, _ *apiv1schema.GetERDiagramRequest) (*apiv1schema.GetERDiagramResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()

	tables, err := meta.Query(ctx, `
		SELECT name FROM lc_tables WHERE tenant_id = $1 AND base_id = $2 ORDER BY name`, tid, baseID)
	if err != nil {
		return nil, err
	}
	defer tables.Close()

	diagram := &apiv1schema.ERDiagram{}

	for tables.Next() {
		var name string
		if err := tables.Scan(&name); err != nil {
			return nil, err
		}
		schemaResp, err := s.GetTableSchema(ctx, &apiv1schema.GetTableSchemaRequest{TableId: name})
		if err != nil {
			return nil, err
		}
		diagram.Nodes = append(diagram.Nodes, &apiv1schema.ERNode{
			TableId:   name,
			TableName: name,
			Label:     name,
			Columns:   schemaResp.Columns,
		})
	}
	if err := tables.Err(); err != nil {
		return nil, err
	}

	colRows, err := meta.Query(ctx, `
		SELECT c.id, c.table_id, c.name, c.type_id, c.config
		FROM lc_columns c
		WHERE c.tenant_id = $1 AND c.base_id = $2
		  AND c.type_id IN ('link', 'relationship', 'relation_fk')`, tid, baseID)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()
	for colRows.Next() {
		var colID, tableID, colName, typeID string
		var cfg map[string]any
		if err := colRows.Scan(&colID, &tableID, &colName, &typeID, &cfg); err != nil {
			return nil, err
		}
		targetTable := shared.CfgString(cfg, "target_table_id")
		if targetTable == "" {
			continue
		}
		edgeKind := "MANY_TO_ONE"
		if (typeID == "link" || typeID == "relationship" || typeID == "relation_fk") && shared.CfgString(cfg, "link_column_id") != "" {
			edgeKind = "ONE_TO_MANY"
		}
		if typeID == "link" && strings.EqualFold(shared.CfgString(cfg, "cardinality"), "many") {
			edgeKind = "ONE_TO_MANY"
		}
		diagram.Edges = append(diagram.Edges, &apiv1schema.EREdge{
			Id:             colID,
			Kind:           edgeKind,
			SourceTableId:  tableID,
			SourceColumnId: colID,
			TargetTableId:  targetTable,
			TargetColumnId: shared.CfgString(cfg, "target_column_id"),
			Label:          colName,
		})
	}

	return &apiv1schema.GetERDiagramResponse{Diagram: diagram}, colRows.Err()
}
