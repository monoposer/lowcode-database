package shared

import (
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/columntype"
	formulacompile "github.com/monoposer/lowcode-database/internal/formula"
	"strings"
)

func CompileFormulaExpression(expr, alias string, nameToPg map[string]string) (string, error) {
	return formulacompile.Compile(expr, alias, nameToPg)
}

// FormulaDefs collects non-empty formula column definitions from table metadata.
func FormulaDefs(cols []FullColumnMeta) []formulacompile.Def {
	var defs []formulacompile.Def
	for _, c := range cols {
		if c.Kind != "formula" {
			continue
		}
		expr := FormulaExpression(c.Config)
		if expr == "" {
			continue
		}
		defs = append(defs, formulacompile.Def{Name: c.Name, Expr: expr})
	}
	return defs
}

// FormulaRefAllowed reports whether {{column_name}} may reference this column in a formula.
func FormulaRefAllowed(kind string) bool {
	switch kind {
	case "formula", "lookup", "rollup":
		return true
	default:
		return !IsVirtualKind(kind)
	}
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

// LookupManyAggregateSQL builds a correlated subquery that array_agg's a column from related rows (cardinality many).
func LookupManyAggregateSQL(valueExpr, linkPgCol, targetSchema, targetTable, baseAlias, extraWhere, extraFrom, arrayPgType string) string {
	schemaQ := pgx.Identifier{targetSchema}.Sanitize()
	tableQ := pgx.Identifier{targetTable}.Sanitize()
	linkQ := pgx.Identifier{linkPgCol}.Sanitize()
	baseQ := pgx.Identifier{baseAlias}.Sanitize()
	where := fmt.Sprintf(`_r.%s = %s.id`, linkQ, baseQ)
	if strings.TrimSpace(extraWhere) != "" {
		where += " AND (" + extraWhere + ")"
	}
	cast := strings.TrimSpace(arrayPgType)
	if cast == "" {
		cast = "text[]"
	}
	fromExtra := extraFrom
	return fmt.Sprintf(`(
		SELECT COALESCE(array_agg(%s ORDER BY _r.id), '{}'::%s)
		FROM %s.%s AS _r%s
		WHERE %s
	)`, valueExpr, cast, schemaQ, tableQ, fromExtra, where)
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

// LookupTargetAllowed reports whether a column on the related table may be used as lookup target_column_id.
func LookupTargetAllowed(kind string) bool {
	switch kind {
	case "formula", "lookup", "rollup":
		return true
	default:
		return !IsVirtualKind(kind)
	}
}

// LookupWriteSpec describes how to resolve a lookup column value into a local FK column on write.
type LookupWriteSpec struct {
	LookupName    string
	LocalFKColumn string
	LocalFKPgType string
	TargetTableID string
	TargetSchema  string
	TargetTable   string
	SearchColumn  string
	SearchPgType  string
	RefColumn     string
	RefPgType     string
	Filter        map[string]any
	TargetCols    []ColumnMeta
}
