package shared

import (
	"github.com/monoposer/lowcode-database/internal/numeric"
)

// NormalizeNumberValue applies financial scale rounding to a number cell.
func NormalizeNumberValue(v *Value, spec numeric.Spec) *Value {
	if v == nil || v.NumberValue == nil || !spec.FinancialMode {
		return v
	}
	if n, ok := numeric.NormalizeAny(*v.NumberValue, spec); ok {
		return NumberValue(n)
	}
	return v
}

// NormalizeNumberAny applies financial rounding to a native JSON value.
func NormalizeNumberAny(raw any, spec numeric.Spec) any {
	if !spec.FinancialMode {
		return raw
	}
	if n, ok := numeric.NormalizeAny(raw, spec); ok {
		return n
	}
	return raw
}
