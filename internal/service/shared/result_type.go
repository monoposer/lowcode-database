package shared

import (
	"fmt"
	"strings"
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
	return strings.HasSuffix(typeID, "_array")
}

// IsNumericResultType reports scalar numeric types used for filters and coercion.
func IsNumericResultType(typeID string) bool {
	switch typeID {
	case "int8", "double", "number", "integer", "numeric", "precision":
		return true
	default:
		return false
	}
}

// IsDateTimeResultType reports scalar date/time types.
func IsDateTimeResultType(typeID string) bool {
	switch typeID {
	case "timestamp", "timestamptz", "date":
		return true
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
	if IsNumericResultType(id) || IsDateTimeResultType(id) {
		return nil
	}
	switch id {
	case "text", "bool", "json", "jsonb", "uuid", "bytea",
		"int8_array", "double_array", "text_array", "bool_array", "jsonb_array",
		"timestamptz_array", "uuid_array", "geometry", "geography", "point":
		return nil
	default:
		return fmt.Errorf("unknown result_type_id %q", id)
	}
}

// -------- Infer --------

// InferFormulaResultTypeId guesses formula return type from expression shape.
func InferFormulaResultTypeId(expr string) string {
	e := strings.TrimSpace(expr)
	if e == "" {
		return "number"
	}
	u := strings.ToUpper(e)
	if strings.Contains(u, "CONCAT(") || strings.Contains(u, "TEXT(") ||
		strings.Contains(u, "LOWER(") || strings.Contains(u, "UPPER(") {
		return "text"
	}
	if strings.ContainsAny(e, "\"'") {
		return "text"
	}
	return "number"
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
	switch scalar {
	case "int8", "integer", "number":
		return "int8_array"
	case "double", "precision", "numeric":
		return "double_array"
	case "bool":
		return "bool_array"
	case "json", "jsonb":
		return "jsonb_array"
	case "uuid":
		return "uuid_array"
	case "timestamptz", "timestamp", "date":
		return "timestamptz_array"
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
