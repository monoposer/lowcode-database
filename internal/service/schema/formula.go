package schema

import (
	"context"

	"github.com/solat/lowcode-database/internal/formula"
	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/shared"
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
	refs := validationFormulaRefs(allCols, columnName)
	_, err = shared.CompileFormulaExpression(expr, "_b", refs)
	return err
}

func (s *Schema) CompileFormulaForTable(ctx context.Context, tableKey, expr string) (string, error) {
	allCols, _, _, err := catalog.New(s.B).LoadAllColumnMeta(ctx, tableKey)
	if err != nil {
		return "", err
	}
	refs := validationFormulaRefs(allCols, "")
	return shared.CompileFormulaExpression(expr, "_b", refs)
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

// validationFormulaRefs builds a ref map for syntax-checking one expression.
// Other formula columns use a numeric stub; the column being edited is omitted.
func validationFormulaRefs(cols []shared.FullColumnMeta, editingName string) map[string]string {
	refs := map[string]string{}
	for _, c := range cols {
		switch {
		case shared.FormulaRefAllowed(c.Kind) && c.Kind != "formula":
			refs[c.Name] = c.Name
		case c.Kind == "formula" && c.Name != editingName:
			refs[c.Name] = formula.StubRef(c.Name)
		}
	}
	return refs
}
