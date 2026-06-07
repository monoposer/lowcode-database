package schema

import (
	"context"

	"github.com/monoposer/lowcode-database/internal/formula"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func (s *Schema) ValidateFormulaExpression(ctx context.Context, tableKey, columnName, expr string) error {
	allCols, _, _, err := catalog.New(s.B).LoadAllColumnMeta(ctx, tableKey)
	if err != nil {
		return err
	}
	formulas := formulaExprsByName(allCols)
	if columnName != "" {
		if err := formula.DetectCycle(formulas, columnName, expr); err != nil {
			return err
		}
	}
	return formula.Validate(expr, knownFormulaColumns(allCols, columnName))
}

func (s *Schema) CompileFormulaForTable(ctx context.Context, tableKey, expr string) (string, error) {
	allCols, _, _, err := catalog.New(s.B).LoadAllColumnMeta(ctx, tableKey)
	if err != nil {
		return "", err
	}
	if err := formula.Validate(expr, knownFormulaColumns(allCols, "")); err != nil {
		return "", err
	}
	return expr, nil
}

func formulaExprsByName(cols []shared.FullColumnMeta) map[string]string {
	out := make(map[string]string)
	for _, c := range cols {
		if c.Kind != "formula" {
			continue
		}
		if e := shared.FormulaExpression(c.Config); e != "" {
			out[c.Name] = e
		}
	}
	return out
}

func knownFormulaColumns(cols []shared.FullColumnMeta, editingName string) map[string]struct{} {
	known := map[string]struct{}{}
	for _, c := range cols {
		if c.Name == editingName {
			continue
		}
		if shared.FormulaRefAllowed(c.Kind) {
			known[c.Name] = struct{}{}
		}
	}
	return known
}
