package data

import (
	"context"
	"fmt"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func (s *Data) buildLookupJoinSpecs(ctx context.Context, tableID string, argAcc *argAccumulator) ([]lookupJoinSpec, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	resolvedName, err := s.B.ResolveTableName(ctx, tableID)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	const q = `
		SELECT c.name, c.config
		FROM lc_columns c
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.type_id = 'lookup'
	`
	rows, err := meta.Query(ctx, q, resolvedName, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aliases := newJoinAliasRegistry()
	var specs []lookupJoinSpec
	for rows.Next() {
		var colName string
		var cfg map[string]any
		if err := rows.Scan(&colName, &cfg); err != nil {
			return nil, err
		}
		spec, ok, err := s.lookupJoinSpecForColumn(ctx, tid, tableID, colName, cfg, argAcc, aliases)
		if err != nil {
			return nil, err
		}
		if ok {
			specs = append(specs, spec)
		}
	}
	return specs, rows.Err()
}

func (s *Data) lookupJoinSpecForColumn(
	ctx context.Context,
	tid, tableID, colName string,
	cfg map[string]any,
	argAcc *argAccumulator,
	aliases *joinAliasRegistry,
) (lookupJoinSpec, bool, error) {
	relID := shared.CfgString(cfg, "relation_column_id")
	fieldID := shared.CfgString(cfg, "target_column_id")
	if relID == "" || fieldID == "" {
		return lookupJoinSpec{}, false, nil
	}
	rels, err := s.meta().LoadRelationshipColumns(ctx, tableID, []string{relID})
	if err != nil || len(rels) == 0 {
		return lookupJoinSpec{}, false, nil
	}
	rel := rels[0]
	tgtSchema, tgtTable, err := s.tableSchemaName(ctx, rel.TargetTableId)
	if err != nil {
		return lookupJoinSpec{}, false, nil
	}
	var filter map[string]any
	if raw, ok := cfg["filter"].(map[string]any); ok && len(raw) > 0 {
		filter = raw
	}
	targetCols, _, _, err := s.meta().LoadColumns(ctx, rel.TargetTableId)
	if err != nil {
		return lookupJoinSpec{}, false, nil
	}

	if rel.Cardinality == "many" && rel.LinkColumnId != "" {
		spec, err := s.buildManyLookupJoinSpec(ctx, tid, colName, rel, fieldID, tgtSchema, tgtTable, filter, targetCols, argAcc)
		if err != nil {
			return lookupJoinSpec{}, false, err
		}
		return spec, true, nil
	}
	if rel.Cardinality != "one" || rel.TargetColumnId == "" {
		return lookupJoinSpec{}, false, nil
	}
	spec, err := s.buildOneLookupJoinSpec(ctx, tid, tableID, colName, rel, fieldID, tgtSchema, tgtTable, filter, targetCols, argAcc, aliases)
	if err != nil {
		return lookupJoinSpec{}, false, err
	}
	return spec, true, nil
}

type lookupJoinSpec struct {
	LookupColumnName  string
	Cardinality       string
	Alias             string
	TargetSchema      string
	TargetTable       string
	SelectExpr        string
	TargetValuePgType string
	BaseFKPgCol       string
	LinkPgCol         string
	Filter            map[string]any
	TargetCols        []shared.ColumnMeta
	ExtraFromSQL      string
}

func (s *Data) buildManyLookupJoinSpec(
	ctx context.Context,
	tid, colName string,
	rel shared.RelationshipColumn,
	fieldID, tgtSchema, tgtTable string,
	filter map[string]any,
	targetCols []shared.ColumnMeta,
	argAcc *argAccumulator,
) (lookupJoinSpec, error) {
	linkPg, err := s.meta().ColumnPgColumnByRef(ctx, tid, rel.TargetTableId, rel.LinkColumnId)
	if err != nil {
		return lookupJoinSpec{}, err
	}
	aliases := newJoinAliasRegistry()
	resolved, err := s.resolveLookupTargetValue(ctx, rel.TargetTableId, fieldID, "_r", argAcc, map[string]bool{}, aliases)
	if err != nil {
		return lookupJoinSpec{}, fmt.Errorf("lookup %q: %w", colName, err)
	}
	extraWhere := ""
	if len(filter) > 0 && len(targetCols) > 0 {
		wSQL, wArgs, err := linkedTableFilterSQL(map[string]any{"filter": filter}, "_r", targetCols, argAcc.nextArgStart())
		if err != nil {
			return lookupJoinSpec{}, fmt.Errorf("lookup %q filter: %w", colName, err)
		}
		if wSQL != "" {
			extraWhere = wSQL
			argAcc.append(wArgs...)
		}
	}
	arrayPgType := shared.ScalarPgTypeToArray(resolved.PgType)
	selectExpr := shared.LookupManyAggregateSQL(
		resolved.SelectExpr, linkPg, tgtSchema, tgtTable, "_b", extraWhere, resolved.ExtraFrom, arrayPgType,
	)
	return lookupJoinSpec{
		LookupColumnName:  colName,
		Cardinality:       "many",
		TargetSchema:      tgtSchema,
		TargetTable:       tgtTable,
		SelectExpr:        selectExpr,
		TargetValuePgType: arrayPgType,
		LinkPgCol:         linkPg,
		Filter:            filter,
		TargetCols:        targetCols,
	}, nil
}

func (s *Data) buildOneLookupJoinSpec(
	ctx context.Context,
	tid, tableID, colName string,
	rel shared.RelationshipColumn,
	fieldID, tgtSchema, tgtTable string,
	filter map[string]any,
	targetCols []shared.ColumnMeta,
	argAcc *argAccumulator,
	aliases *joinAliasRegistry,
) (lookupJoinSpec, error) {
	baseFKPg, err := s.meta().ColumnPgColumnByRef(ctx, tid, tableID, rel.TargetColumnId)
	if err != nil {
		return lookupJoinSpec{}, err
	}
	alias := aliases.sharedRelRowAlias(rel.Id)
	resolved, err := s.resolveLookupTargetValue(ctx, rel.TargetTableId, fieldID, alias, argAcc, map[string]bool{}, aliases)
	if err != nil {
		return lookupJoinSpec{}, fmt.Errorf("lookup %q: %w", colName, err)
	}
	tgtValuePgType := resolved.PgType
	if tgtValuePgType == "" {
		for _, tc := range targetCols {
			if tc.Name == fieldID {
				tgtValuePgType = tc.PgType
				break
			}
		}
	}
	allTarget, _, _, _ := s.meta().LoadAllColumnMeta(ctx, rel.TargetTableId)
	for _, tc := range allTarget {
		if tc.Name == fieldID && tc.PgType != "" {
			tgtValuePgType = tc.PgType
			break
		}
	}
	return lookupJoinSpec{
		LookupColumnName:  colName,
		Cardinality:       "one",
		Alias:             alias,
		TargetSchema:      tgtSchema,
		TargetTable:       tgtTable,
		SelectExpr:        resolved.SelectExpr,
		TargetValuePgType: tgtValuePgType,
		BaseFKPgCol:       baseFKPg,
		Filter:            filter,
		TargetCols:        targetCols,
		ExtraFromSQL:      resolved.ExtraFrom,
	}, nil
}
