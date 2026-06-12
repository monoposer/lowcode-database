package schema

import (
	"context"
	"fmt"
	apiv1schema "github.com/solat/lowcode-database/internal/apiv1/schema"
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

// -------- ER Diagram --------

func (s *Schema) GetERDiagram(ctx context.Context, _ *apiv1schema.GetERDiagramRequest) (*apiv1schema.GetERDiagramResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()

	tables, err := meta.Query(ctx, `
		SELECT name, schema_name FROM lc_tables WHERE tenant_id = $1 ORDER BY name`, tid)
	if err != nil {
		return nil, err
	}
	defer tables.Close()

	diagram := &apiv1schema.ERDiagram{}

	for tables.Next() {
		var name, schemaName string
		if err := tables.Scan(&name, &schemaName); err != nil {
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

	relRows, err := meta.Query(ctx, `
		SELECT name, kind, source_table_id, source_column_id, target_table_id, target_column_id
		FROM lc_relations WHERE tenant_id = $1`, tid)
	if err != nil {
		return nil, err
	}
	defer relRows.Close()
	for relRows.Next() {
		var e apiv1schema.EREdge
		var relName string
		var srcCol, tgtCol *string
		if err := relRows.Scan(&relName, &e.Kind, &e.SourceTableId, &srcCol, &e.TargetTableId, &tgtCol); err != nil {
			return nil, err
		}
		e.Id = relName
		e.Label = relName
		if srcCol != nil {
			e.SourceColumnId = *srcCol
		}
		if tgtCol != nil {
			e.TargetColumnId = *tgtCol
		}
		diagram.Edges = append(diagram.Edges, &e)
	}
	if err := relRows.Err(); err != nil {
		return nil, err
	}

	colRows, err := meta.Query(ctx, `
		SELECT c.id, c.table_id, c.name, c.type_id, c.config
		FROM lc_columns c
		WHERE c.tenant_id = $1
		  AND c.type_id IN ('relationship', 'relation_fk')`, tid)
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
		if typeID == "relationship" && shared.CfgString(cfg, "link_column_id") != "" {
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
