package shared

import (
	"fmt"
	"strings"

	"github.com/monoposer/lowcode-database/internal/columntype"
)

// -------- Classify --------

const ConfigKeyResultTypeID = "result_type_id"

// ConfigResultTypeID reads persisted value-type metadata from column config.
func ConfigResultTypeID(cfg map[string]any) string {
	return CfgString(cfg, ConfigKeyResultTypeID)
}

// SetConfigResultTypeID stores value-type metadata in column config (mutates cfg).
func SetConfigResultTypeID(cfg map[string]any, resultTypeID string) {
	if cfg == nil || strings.TrimSpace(resultTypeID) == "" {
		return
	}
	cfg[ConfigKeyResultTypeID] = strings.TrimSpace(resultTypeID)
}

// IsNumericResultType reports scalar numeric types used for filters and coercion.
func IsNumericResultType(typeID string) bool {
	return columntype.CanonicalID(typeID) == "number"
}

// IsDateTimeResultType reports scalar date/time types.
func IsDateTimeResultType(typeID string) bool {
	return columntype.CanonicalID(typeID) == "datetime"
}

// ValidateResultTypeID checks that id is a known built-in scalar or array type id.
func ValidateResultTypeID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("result_type_id is empty")
	}
	t, ok := columntype.GetPgType(id)
	if !ok || t.Kind != "" {
		return fmt.Errorf("unknown result_type_id %q", id)
	}
	return nil
}

// -------- Infer --------

// InferFormulaResultTypeId guesses formula return type from expression shape.
func InferFormulaResultTypeId(expr string) string {
	e := strings.TrimSpace(expr)
	if e == "" {
		return "number"
	}
	u := strings.ToUpper(e)
	switch {
	case hasCall(u, "ISBLANK"):
		return "boolean"
	case hasCall(u, "DATEDIF"), hasCall(u, "YEAR"), hasCall(u, "MONTH"), hasCall(u, "DAY"), hasCall(u, "LEN"),
		hasCall(u, "MIN"), hasCall(u, "MAX"), hasCall(u, "INT"), hasCall(u, "WEEKDAY"):
		return "number"
	case hasCall(u, "CONCAT"), hasCall(u, "TEXT"), hasCall(u, "LOWER"), hasCall(u, "UPPER"), hasCall(u, "TRIM"):
		return "text"
	case hasCall(u, "DATEVALUE"), hasCall(u, "DATE"), hasCall(u, "TODAY"), hasCall(u, "NOW"):
		return "datetime"
	case hasCall(u, "MID"):
		return "text"
	case strings.ContainsAny(e, "\"'"):
		return "text"
	default:
		return "number"
	}
}

func hasCall(upperExpr, name string) bool {
	needle := name + "("
	from := 0
	for {
		i := strings.Index(upperExpr[from:], needle)
		if i < 0 {
			return false
		}
		i += from
		if i == 0 || !isIdentChar(upperExpr[i-1]) {
			return true
		}
		from = i + 1
	}
}

func isIdentChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// RollupResultTypeId derives rollup value type from aggregate and target field type.
func RollupResultTypeId(aggregate, targetResultType string) string {
	agg := strings.ToLower(strings.TrimSpace(aggregate))
	target := strings.TrimSpace(targetResultType)
	if target == "" {
		target = "number"
	}
	switch agg {
	case "count":
		return "number"
	case "sum", "avg":
		if IsNumericResultType(target) {
			return "number"
		}
		return target
	case "min", "max":
		return target
	default:
		return "number"
	}
}

// LookupManyResultTypeID returns the result type id for a many-cardinality lookup (scalar id;
// array-ness is expressed via the effective PgType suffix [], not a separate type id).
func LookupManyResultTypeID(scalar string) string {
	return columntype.CanonicalID(scalar)
}

// ScalarPgTypeToArray maps a scalar PostgreSQL type to an array pg type for array_agg.
func ScalarPgTypeToArray(pgType string) string {
	pgType = strings.TrimSpace(pgType)
	if strings.HasSuffix(pgType, "[]") {
		return pgType
	}
	switch strings.ToLower(pgType) {
	case "bigint", "int8":
		return "bigint[]"
	case "double precision", "float8":
		return "double precision[]"
	case "boolean", "bool":
		return "boolean[]"
	case "uuid":
		return "uuid[]"
	case "timestamptz", "timestamp with time zone":
		return "timestamptz[]"
	case "jsonb", "json":
		return "jsonb[]"
	case "numeric":
		return "numeric[]"
	default:
		return "text[]"
	}
}
