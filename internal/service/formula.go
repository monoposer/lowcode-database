package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/solat/lowcode-database/internal/columntype"
	formulacompile "github.com/solat/lowcode-database/internal/formula"
)

// compileFormulaExpression converts Excel-style expressions to PostgreSQL via pg-formula.
func compileFormulaExpression(expr, alias string, nameToPg map[string]string) (string, error) {
	return formulacompile.Compile(expr, alias, nameToPg)
}

func (s *LowcodeService) validateFormulaExpression(ctx context.Context, tableKey, expr string) error {
	_, err := s.compileFormulaForTable(ctx, tableKey, expr)
	return err
}

func (s *LowcodeService) compileFormulaForTable(ctx context.Context, tableKey, expr string) (string, error) {
	allCols, _, _, err := s.loadAllColumnMeta(ctx, tableKey)
	if err != nil {
		return "", err
	}
	nameToPg := map[string]string{}
	for _, c := range allCols {
		if !c.IsVirtual {
			nameToPg[c.Name] = c.PgColumn
		}
	}
	return compileFormulaExpression(expr, "_b", nameToPg)
}

func formulaExpression(cfg map[string]any) string {
	if cfg == nil {
		return ""
	}
	if e := cfgString(cfg, "expression"); e != "" {
		return e
	}
	return cfgString(cfg, "formula")
}

// rollupAggregateSQL builds a lateral subquery for rollup columns.
func rollupAggregateSQL(aggregate, targetPgCol, linkPgCol, targetSchema, targetTable, alias string) string {
	fn := strings.ToUpper(strings.TrimSpace(aggregate))
	if fn == "" {
		fn = "COUNT"
	}
	colExpr := "*"
	if targetPgCol != "" && fn != "COUNT" {
		colExpr = targetPgCol
	}
	return fmt.Sprintf(`(
		SELECT %s(%s) FROM %s.%s AS _r
		WHERE _r.%s = %s.id
	)`, fn, colExpr, targetSchema, targetTable, linkPgCol, alias)
}

func isVirtualKind(kind string) bool {
	return columntype.IsVirtualKind(kind)
}

func effectivePgType(pgType string, typeConfig map[string]any) string {
	if typeConfig == nil {
		return pgType
	}
	if p, ok := typeConfig["precision"].(float64); ok {
		scale, _ := typeConfig["scale"].(float64)
		if scale == 0 {
			scale = 6
		}
		return fmt.Sprintf("numeric(%d,%d)", int(p), int(scale))
	}
	return pgType
}
