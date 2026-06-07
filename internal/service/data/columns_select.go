package data

import (
	"context"

	"github.com/monoposer/lowcode-database/internal/service/calc"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// queryableColumnNames lists logical column names exposed by a saved query
// (physical + formula / lookup / rollup; excludes link).
func queryableColumnNames(allCols []shared.FullColumnMeta) []string {
	var names []string
	for _, c := range allCols {
		if calc.IsLinkType(c.Kind) {
			continue
		}
		names = append(names, c.Name)
	}
	return names
}

// resolveQueryProjection returns the column list for a saved query SELECT.
//   - reqCols empty → use query column_names; empty column_names means SELECT * (all queryable table columns)
//   - reqCols set   → intersection with the query projection above
func (s *Data) resolveQueryProjection(ctx context.Context, tableID string, dsCols, reqCols []string) ([]string, error) {
	if len(reqCols) > 0 {
		tid, err := s.B.TenantID(ctx)
		if err != nil {
			return nil, err
		}
		var normErr error
		reqCols, normErr = s.meta().NormalizeColumnNames(ctx, tid, tableID, reqCols)
		if normErr != nil {
			return nil, normErr
		}
	}

	viewCols := dsCols
	if len(viewCols) == 0 {
		allCols, _, _, err := s.meta().LoadAllColumnMeta(ctx, tableID)
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
) {
	for _, lk := range lookupSpecs {
		attrMap[lk.LookupColumnName] = lk.SelectExpr
	}
	for name, sql := range rollupSQLByName {
		attrMap[name] = "(" + sql + ")"
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
