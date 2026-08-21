package numeric

import (
	"github.com/monoposer/lowcode-database/internal/columntype"
)

// SpecFromColumnTypeSpec builds a numeric spec from a columnType spec document.
func SpecFromColumnTypeSpec(spec *columntype.ColumnTypeSpec) Spec {
	if spec == nil || spec.PgType != "number" {
		return Spec{}
	}
	out := DefaultNumberSpec()
	if spec.Precision != nil {
		out.Precision = *spec.Precision
	}
	if spec.Scale != nil {
		out.Scale = *spec.Scale
	}
	if spec.FinancialMode != nil {
		out.FinancialMode = *spec.FinancialMode
	}
	if spec.RoundingMode != "" {
		if mode, err := ParseRoundingMode(spec.RoundingMode); err == nil {
			out.RoundingMode = mode
		}
	}
	return out
}

// SpecFromConfig builds a numeric spec from a type config map (API / builtin registry).
func SpecFromConfig(cfg map[string]any) Spec {
	if cfg == nil {
		return Spec{}
	}
	pg, _ := cfg["pgType"].(string)
	if pg != "" && pg != "number" {
		return Spec{}
	}
	out := DefaultNumberSpec()
	if p, ok := intFromAny(cfg["precision"]); ok {
		out.Precision = p
	}
	if s, ok := intFromAny(cfg["scale"]); ok {
		out.Scale = s
	}
	if fm, ok := cfg["financialMode"].(bool); ok {
		out.FinancialMode = fm
	}
	if rm, ok := cfg["roundingMode"].(string); ok && rm != "" {
		if mode, err := ParseRoundingMode(rm); err == nil {
			out.RoundingMode = mode
		}
	}
	return out
}

// SpecForBuiltin returns numeric spec for a built-in pgType id.
func SpecForBuiltin(typeID string) (Spec, bool) {
	if typeID != "number" {
		return Spec{}, false
	}
	t, ok := columntype.Get(typeID)
	if !ok {
		return Spec{}, false
	}
	return SpecFromConfig(t.Config), true
}

func intFromAny(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int32:
		return int(t), true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	default:
		return 0, false
	}
}

// IsNumericType reports whether typeID is the built-in number pgType.
func IsNumericType(typeID string) bool {
	return typeID == "number"
}
