package schema

import (
	"context"
	"fmt"

	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// LoadLookupWriteSpecs returns lookup columns that can be resolved to a local FK on write (cardinality-one only).
func (s *Schema) LoadLookupWriteSpecs(ctx context.Context, tableName string) (map[string]shared.LookupWriteSpec, error) {
	allCols, schemaName, tableName, err := catalog.New(s.B).LoadAllColumnMeta(ctx, tableName)
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
		rels, err := s.LoadRelationshipColumns(ctx, tableName, []string{relRef})
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

		localFK, err := s.ResolveColumnName(ctx, tid, tableName, rel.TargetColumnId)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: local fk: %w", col.Name, err)
		}
		localFKPgType, err := s.columnPgTypeByName(ctx, tid, tableName, localFK)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: local fk type: %w", col.Name, err)
		}

		refCol, err := s.fkReferencedColumnOnTarget(ctx, tid, tableName, localFK)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: fk ref: %w", col.Name, err)
		}

		tgt, ok := targetCache[rel.TargetTableName]
		if !ok {
			var err error
			tgt.cols, tgt.schema, tgt.table, err = catalog.New(s.B).LoadColumns(ctx, rel.TargetTableName)
			if err != nil {
				return nil, err
			}
			targetCache[rel.TargetTableName] = tgt
		}

		searchCol, err := s.ResolveColumnName(ctx, tid, rel.TargetTableName, fieldRef)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: search column: %w", col.Name, err)
		}
		searchPgType, err := s.columnPgTypeByName(ctx, tid, rel.TargetTableName, searchCol)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: search column type: %w", col.Name, err)
		}
		refPgType, err := s.columnPgTypeByName(ctx, tid, rel.TargetTableName, refCol)
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
			TargetTableName: rel.TargetTableName,
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

func (s *Schema) columnPgTypeByName(ctx context.Context, tenantID, tableName, colName string) (string, error) {
	resolvedTable, err := s.B.ResolveTableName(ctx, tableName)
	if err != nil {
		return "", err
	}
	var typeID string
	var cfg map[string]any
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return "", err
	}
	err = s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT type_id, config FROM lc_columns
		WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3 AND name = $4`,
		tenantID, baseID, resolvedTable, colName,
	).Scan(&typeID, &cfg)
	if err != nil {
		return "", err
	}
	return catalog.New(s.B).ColumnPgTypeSQL(ctx, tenantID, typeID, cfg), nil
}

func (s *Schema) fkReferencedColumnOnTarget(ctx context.Context, tenantID, tableName, fkColName string) (string, error) {
	resolvedTable, err := s.B.ResolveTableName(ctx, tableName)
	if err != nil {
		return "", err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return "", err
	}
	var typeID string
	var cfg map[string]any
	err = s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT type_id, config FROM lc_columns
		WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3 AND name = $4`,
		tenantID, baseID, resolvedTable, fkColName,
	).Scan(&typeID, &cfg)
	if err != nil {
		return "", err
	}
	if typeID != "link" {
		return "id", nil
	}
	ref := shared.CfgString(cfg, "target_column_id")
	if ref == "" {
		return "id", nil
	}
	targetTable := shared.CfgString(cfg, "target_table_name")
	if targetTable == "" {
		return "id", nil
	}
	return s.ResolveColumnName(ctx, tenantID, targetTable, ref)
}
