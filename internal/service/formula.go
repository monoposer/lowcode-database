package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/solat/lowcode-database/internal/columntype"
)

var formulaVarRe = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

// compileFormulaExpression converts {{column_name}} templates to SQL referencing alias.col.
func compileFormulaExpression(expr, alias string, nameToPg map[string]string) (string, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "NULL", nil
	}
	var err error
	out := formulaVarRe.ReplaceAllStringFunc(expr, func(m string) string {
		if err != nil {
			return m
		}
		sub := formulaVarRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			err = fmt.Errorf("invalid formula reference %q", m)
			return m
		}
		pg, ok := nameToPg[sub[1]]
		if !ok {
			err = fmt.Errorf("formula references unknown column %q", sub[1])
			return m
		}
		return alias + "." + pg
	})
	if err != nil {
		return "", err
	}
	return out, nil
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
