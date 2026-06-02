package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/service/schema"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// resolveLookupCells maps lookup column values to local FK columns (saveGraph and similar write paths).
// When both lookup and FK column are present, the explicit FK value wins.
func (s *Data) resolveLookupCells(ctx context.Context, tx pgx.Tx, tableID string, cells map[string]*apiv1.Value) (map[string]*apiv1.Value, error) {
	if len(cells) == 0 {
		return cells, nil
	}
	specs, err := schema.New(s.B).LoadLookupWriteSpecs(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return cells, nil
	}

	out := make(map[string]*apiv1.Value, len(cells))
	for k, v := range cells {
		out[k] = v
	}

	for lookupName, spec := range specs {
		lv, ok := out[lookupName]
		if !ok || lv == nil {
			continue
		}
		if _, hasFK := out[spec.LocalFKColumn]; hasFK {
			delete(out, lookupName)
			continue
		}
		fkVal, err := s.resolveOneLookupValueTx(ctx, tx, spec, lv)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: %w", lookupName, err)
		}
		out[spec.LocalFKColumn] = fkVal
		delete(out, lookupName)
	}
	return out, nil
}

func (s *Data) resolveOneLookupValueTx(ctx context.Context, tx pgx.Tx, spec shared.LookupWriteSpec, lookupVal *apiv1.Value) (*apiv1.Value, error) {
	searchArg := shared.ValueToAnyForColumn(lookupVal, spec.SearchPgType)
	if searchArg == nil {
		return nil, fmt.Errorf("lookup value is required")
	}

	q := fmt.Sprintf(`SELECT t.%s FROM %s.%s AS t WHERE t.%s = $1`,
		pgx.Identifier{spec.RefColumn}.Sanitize(),
		pgx.Identifier{spec.TargetSchema}.Sanitize(),
		pgx.Identifier{spec.TargetTable}.Sanitize(),
		pgx.Identifier{spec.SearchColumn}.Sanitize(),
	)
	args := []any{searchArg}
	argIdx := 2
	if len(spec.Filter) > 0 && len(spec.TargetCols) > 0 {
		filterSQL, filterArgs, err := linkedTableFilterSQL(map[string]any{"filter": spec.Filter}, "t", spec.TargetCols, argIdx)
		if err != nil {
			return nil, err
		}
		if filterSQL != "" {
			q += " AND (" + filterSQL + ")"
			args = append(args, filterArgs...)
		}
	}
	q += " LIMIT 2"

	rows, err := tx.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []any
	for rows.Next() {
		var v any
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		matches = append(matches, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no %s row matches %s = %v", spec.TargetTable, spec.SearchColumn, searchArg)
	case 1:
		return shared.DBCellValue(matches[0], spec.LocalFKPgType), nil
	default:
		return nil, fmt.Errorf("ambiguous %s match for %s = %v", spec.TargetTable, spec.SearchColumn, searchArg)
	}
}
