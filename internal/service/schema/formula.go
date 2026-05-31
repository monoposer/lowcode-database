package schema

import (
	"context"

	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/shared"
)

func (s *Schema) ValidateFormulaExpression(ctx context.Context, tableKey, expr string) error {
	_, err := s.CompileFormulaForTable(ctx, tableKey, expr)
	return err
}

func (s *Schema) CompileFormulaForTable(ctx context.Context, tableKey, expr string) (string, error) {
	allCols, _, _, err := catalog.New(s.B).LoadAllColumnMeta(ctx, tableKey)
	if err != nil {
		return "", err
	}
	nameToPg := map[string]string{}
	for _, c := range allCols {
		if !c.IsVirtual {
			nameToPg[c.Name] = c.Name
		}
	}
	return shared.CompileFormulaExpression(expr, "_b", nameToPg)
}
