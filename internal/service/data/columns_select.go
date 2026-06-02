package data

import (
	"context"

	formulacompile "github.com/solat/lowcode-database/internal/formula"
	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/schema"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// queryableColumnNames lists logical column names exposed by a data-source "view"
// (physical + formula / lookup / rollup; excludes relationship).
func queryableColumnNames(allCols []shared.FullColumnMeta) []string {
	var names []string
	for _, c := range allCols {
		if c.Kind == "relationship" {
			continue
		}
		names = append(names, c.Name)
	}
	return names
}

// resolveDataSourceViewProjection returns the column list for SELECT … FROM view.
//   - reqCols empty → use data source column_names; empty column_names on DS means SELECT * (all queryable table columns)
//   - reqCols set   → intersection with the view projection above
func (s *Data) resolveDataSourceViewProjection(ctx context.Context, tableID string, dsCols, reqCols []string) ([]string, error) {
	if len(reqCols) > 0 {
		tid, err := s.B.TenantID(ctx)
		if err != nil {
			return nil, err
		}
		var normErr error
		reqCols, normErr = schema.New(s.B).NormalizeColumnNames(ctx, tid, tableID, reqCols)
		if normErr != nil {
			return nil, normErr
		}
	}

	viewCols := dsCols
	if len(viewCols) == 0 {
		allCols, _, _, err := catalog.New(s.B).LoadAllColumnMeta(ctx, tableID)
		if err != nil {
			return nil, err
		}
		viewCols = queryableColumnNames(allCols)
	}

	if len(reqCols) == 0 {
		return viewCols, nil
	}

	viewSet := make(map[string]struct{}, len(viewCols))
	for _, n := range viewCols {
		viewSet[n] = struct{}{}
	}
	out := make([]string, 0, len(reqCols))
	for _, n := range reqCols {
		if _, ok := viewSet[n]; ok {
			out = append(out, n)
		}
	}
	return out, nil
}

func columnAllowSet(colNames []string) map[string]struct{} {
	m := make(map[string]struct{}, len(colNames))
	for _, n := range colNames {
		m[n] = struct{}{}
	}
	return m
}

func columnAllowed(name string, allow map[string]struct{}) bool {
	_, ok := allow[name]
	return ok
}

// extendAttrMapVirtual adds formula / lookup / rollup columns for filter and ORDER BY.
func extendAttrMapVirtual(
	attrMap map[string]string,
	allCols []shared.FullColumnMeta,
	lookupSpecs []lookupJoinSpec,
	rollupSQLByName map[string]string,
	formulaSteps []formulacompile.Step,
) {
	for _, lk := range lookupSpecs {
		attrMap[lk.LookupColumnName] = lk.SelectExpr
	}
	for name, sql := range rollupSQLByName {
		attrMap[name] = "(" + sql + ")"
	}
	for _, step := range formulaSteps {
		attrMap[step.Name] = step.SelectRef()
	}
	for _, c := range allCols {
		switch c.Kind {
		case "formula", "lookup", "rollup":
			if ref, ok := attrMap[c.Name]; ok {
				attrMap[c.Id] = ref
			}
		}
	}
}
