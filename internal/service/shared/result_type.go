package shared

import (
	"fmt"
	"strings"

	"github.com/monoposer/lowcode-database/pkg/typespec"
)

// -------- Classify --------
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

// IsArrayResultType reports whether the type id denotes a PostgreSQL array column.
func IsArrayResultType(typeID string) bool {
	return typespec.IsArrayType(typeID) || strings.HasSuffix(typeID, "_array")
}

// IsNumericResultType reports scalar numeric types used for filters and coercion.
func IsNumericResultType(typeID string) bool {
	switch typespec.CanonicalID(typeID) {
	case "number":
		return !IsArrayResultType(typeID)
	default:
		return false
	}
}

// IsDateTimeResultType reports scalar date/time types.
func IsDateTimeResultType(typeID string) bool {
	switch typespec.CanonicalID(typeID) {
	case "datetime":
		return !IsArrayResultType(typeID)
	default:
		return false
	}
}

// ValidateResultTypeID checks that id is a known built-in scalar or array type id.
func ValidateResultTypeID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("result_type_id is empty")
	}
	t, ok := typespec.GetPgType(id)
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

// ScalarResultTypeToArray maps a scalar result type id to its array counterpart (many-cardinality lookup).
func ScalarResultTypeToArray(scalar string) string {
	if IsArrayResultType(scalar) {
		return scalar
	}
	switch typespec.CanonicalID(scalar) {
	case "number":
		return "number_array"
	case "boolean":
		return "boolean_array"
	case "jsonb":
		return "jsonb_array"
	case "datetime":
		return "datetime_array"
	case "point":
		return "point_array"
	default:
		return "text_array"
	}
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
	default:
		return "text[]"
	}
}
