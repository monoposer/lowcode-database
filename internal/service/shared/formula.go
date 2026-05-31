package shared

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/columntype"
	formulacompile "github.com/solat/lowcode-database/internal/formula"
)

func CompileFormulaExpression(expr, alias string, nameToPg map[string]string) (string, error) {
	return formulacompile.Compile(expr, alias, nameToPg)
}

func FormulaExpression(cfg map[string]any) string {
	if cfg == nil {
		return ""
	}
	if e := CfgString(cfg, "expression"); e != "" {
		return e
	}
	return CfgString(cfg, "formula")
}

// RollupAggregateSQL builds a correlated subquery for rollup columns.
// extraWhere is appended with AND (e.g. filter on linked rows); identifiers are quoted for camelCase names.
func RollupAggregateSQL(aggregate, targetPgCol, linkPgCol, targetSchema, targetTable, baseAlias, extraWhere string) string {
	fn := strings.ToUpper(strings.TrimSpace(aggregate))
	if fn == "" {
		fn = "COUNT"
	}
	colExpr := "*"
	if targetPgCol != "" && fn != "COUNT" {
		colExpr = pgx.Identifier{targetPgCol}.Sanitize()
	}
	schemaQ := pgx.Identifier{targetSchema}.Sanitize()
	tableQ := pgx.Identifier{targetTable}.Sanitize()
	linkQ := pgx.Identifier{linkPgCol}.Sanitize()
	baseQ := pgx.Identifier{baseAlias}.Sanitize()
	where := fmt.Sprintf(`_r.%s = %s.id`, linkQ, baseQ)
	if strings.TrimSpace(extraWhere) != "" {
		where += " AND (" + extraWhere + ")"
	}
	return fmt.Sprintf(`(
		SELECT %s(%s) FROM %s.%s AS _r
		WHERE %s
	)`, fn, colExpr, schemaQ, tableQ, where)
}

func IsVirtualKind(kind string) bool {
	return columntype.IsVirtualKind(kind)
}

func EffectivePgType(pgType string, typeConfig map[string]any) string {
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
