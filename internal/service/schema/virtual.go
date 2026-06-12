package schema

import (
	"context"
	"fmt"

	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// LoadLookupWriteSpecs returns lookup columns that can be resolved to a local FK on write (cardinality-one only).
func (s *Schema) LoadLookupWriteSpecs(ctx context.Context, tableID string) (map[string]shared.LookupWriteSpec, error) {
	allCols, schemaName, tableName, err := catalog.New(s.B).LoadAllColumnMeta(ctx, tableID)
	if err != nil {
		return nil, err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	targetCache := map[string]struct {
		schema string
		table  string
		cols   []shared.ColumnMeta
	}{}

	out := make(map[string]shared.LookupWriteSpec)
	for _, col := range allCols {
		if col.Kind != "lookup" {
			continue
		}
		relRef := shared.CfgString(col.Config, "relation_column_id")
		fieldRef := shared.CfgString(col.Config, "target_column_id")
		if relRef == "" || fieldRef == "" {
			continue
		}
		rels, err := s.LoadRelationshipColumns(ctx, tableID, []string{relRef})
		if err != nil {
			return nil, err
		}
		if len(rels) == 0 {
			continue
		}
		rel := rels[0]
		if rel.Cardinality != "one" || rel.TargetColumnId == "" {
			continue
		}

		localFK, err := s.ResolveColumnName(ctx, tid, tableID, rel.TargetColumnId)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: local fk: %w", col.Name, err)
		}
		localFKPgType, err := s.columnPgTypeByName(ctx, tid, tableID, localFK)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: local fk type: %w", col.Name, err)
		}

		refCol, err := s.fkReferencedColumnOnTarget(ctx, tid, tableID, localFK)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: fk ref: %w", col.Name, err)
		}

		tgt, ok := targetCache[rel.TargetTableId]
		if !ok {
			var err error
			tgt.cols, tgt.schema, tgt.table, err = catalog.New(s.B).LoadColumns(ctx, rel.TargetTableId)
			if err != nil {
				return nil, err
			}
			targetCache[rel.TargetTableId] = tgt
		}

		searchCol, err := s.ResolveColumnName(ctx, tid, rel.TargetTableId, fieldRef)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: search column: %w", col.Name, err)
		}
		searchPgType, err := s.columnPgTypeByName(ctx, tid, rel.TargetTableId, searchCol)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: search column type: %w", col.Name, err)
		}
		refPgType, err := s.columnPgTypeByName(ctx, tid, rel.TargetTableId, refCol)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: ref column type: %w", col.Name, err)
		}

		var filter map[string]any
		if raw, ok := col.Config["filter"].(map[string]any); ok && len(raw) > 0 {
			filter = raw
		}

		out[col.Name] = shared.LookupWriteSpec{
			LookupName:    col.Name,
			LocalFKColumn: localFK,
			LocalFKPgType: localFKPgType,
			TargetTableID: rel.TargetTableId,
			TargetSchema:  tgt.schema,
			TargetTable:   tgt.table,
			SearchColumn:  searchCol,
			SearchPgType:  searchPgType,
			RefColumn:     refCol,
			RefPgType:     refPgType,
			Filter:        filter,
			TargetCols:    tgt.cols,
		}
	}
	_ = schemaName
	_ = tableName
	return out, nil
}

func (s *Schema) columnPgTypeByName(ctx context.Context, tenantID, tableID, colName string) (string, error) {
	resolvedTable, err := s.B.ResolveTableName(ctx, tableID)
	if err != nil {
		return "", err
	}
	var typeID string
	var cfg map[string]any
	err = s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT type_id, config FROM lc_columns
		WHERE tenant_id = $1 AND table_id = $2 AND name = $3`,
		tenantID, resolvedTable, colName,
	).Scan(&typeID, &cfg)
	if err != nil {
		return "", err
	}
	return catalog.New(s.B).ColumnPgTypeSQL(ctx, tenantID, typeID, cfg), nil
}

func (s *Schema) fkReferencedColumnOnTarget(ctx context.Context, tenantID, tableID, fkColName string) (string, error) {
	resolvedTable, err := s.B.ResolveTableName(ctx, tableID)
	if err != nil {
		return "", err
	}
	var typeID string
	var cfg map[string]any
	err = s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT type_id, config FROM lc_columns
		WHERE tenant_id = $1 AND table_id = $2 AND name = $3`,
		tenantID, resolvedTable, fkColName,
	).Scan(&typeID, &cfg)
	if err != nil {
		return "", err
	}
	if columntype.Kind(typeID) != "relation_fk" {
		return "id", nil
	}
	ref := shared.CfgString(cfg, "target_column_id")
	if ref == "" {
		return "id", nil
	}
	targetTable := shared.CfgString(cfg, "target_table_id")
	if targetTable == "" {
		return "id", nil
	}
	return s.ResolveColumnName(ctx, tenantID, targetTable, ref)
}
