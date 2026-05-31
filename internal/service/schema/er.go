package schema

import (
	"context"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// -------- ER Diagram --------

func (s *Schema) GetERDiagram(ctx context.Context, _ *apiv1.GetERDiagramRequest) (*apiv1.GetERDiagramResponse, error) {
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

	diagram := &apiv1.ERDiagram{}

	for tables.Next() {
		var name, schemaName string
		if err := tables.Scan(&name, &schemaName); err != nil {
			return nil, err
		}
		schemaResp, err := s.GetTableSchema(ctx, &apiv1.GetTableSchemaRequest{TableId: name})
		if err != nil {
			return nil, err
		}
		diagram.Nodes = append(diagram.Nodes, &apiv1.ERNode{
			TableId: name,
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
		var e apiv1.EREdge
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
		diagram.Edges = append(diagram.Edges, &apiv1.EREdge{
			Id:             colID,
			Kind:           edgeKind,
			SourceTableId:  tableID,
			SourceColumnId: colID,
			TargetTableId:  targetTable,
			TargetColumnId: shared.CfgString(cfg, "target_column_id"),
			Label:          colName,
		})
	}

	return &apiv1.GetERDiagramResponse{Diagram: diagram}, colRows.Err()
}
