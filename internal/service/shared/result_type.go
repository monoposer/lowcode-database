package shared

import (
	"fmt"
	"strings"
)

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
