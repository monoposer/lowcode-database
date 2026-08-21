package numeric

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

// RoundingMode controls how values are quantized to scale.
type RoundingMode string

const (
	RoundHalfUp   RoundingMode = "half_up"
	RoundHalfEven RoundingMode = "half_even"
	RoundCeil     RoundingMode = "ceil"
	RoundFloor    RoundingMode = "floor"
	RoundTruncate RoundingMode = "truncate"
)

// Spec describes financial numeric handling for a column type.
type Spec struct {
	FinancialMode bool
	Precision     int
	Scale         int
	RoundingMode  RoundingMode
}

// DefaultNumberSpec is the built-in number pgType financial defaults.
func DefaultNumberSpec() Spec {
	return Spec{
		FinancialMode: true,
		Precision:     20,
		Scale:         6,
		RoundingMode:  RoundHalfUp,
	}
}

// ParseRoundingMode normalizes a rounding mode string.
func ParseRoundingMode(s string) (RoundingMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(RoundHalfUp):
		return RoundHalfUp, nil
	case string(RoundHalfEven):
		return RoundHalfEven, nil
	case string(RoundCeil):
		return RoundCeil, nil
	case string(RoundFloor):
		return RoundFloor, nil
	case string(RoundTruncate):
		return RoundTruncate, nil
	default:
		return "", fmt.Errorf("unsupported roundingMode %q", s)
	}
}

// FromAny parses a numeric value from API / JSON / calc env values.
func FromAny(v any) (decimal.Decimal, bool) {
	if v == nil {
		return decimal.Zero, false
	}
	switch t := v.(type) {
	case decimal.Decimal:
		return t, true
	case float64:
		return decimal.NewFromFloat(t), true
	case float32:
		return decimal.NewFromFloat32(t), true
	case int:
		return decimal.NewFromInt(int64(t)), true
	case int32:
		return decimal.NewFromInt32(t), true
	case int64:
		return decimal.NewFromInt(t), true
	case json.Number:
		d, err := decimal.NewFromString(t.String())
		return d, err == nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return decimal.Zero, false
		}
		d, err := decimal.NewFromString(s)
		return d, err == nil
	case bool:
		if t {
			return decimal.NewFromInt(1), true
		}
		return decimal.Zero, true
	default:
		return decimal.Zero, false
	}
}

// ToFloat64 converts a decimal to float64 for JSON API compatibility.
func ToFloat64(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}

// Round applies scale and rounding mode from spec. Non-financial specs pass through.
func Round(d decimal.Decimal, spec Spec) decimal.Decimal {
	if !spec.FinancialMode || spec.Scale < 0 {
		return d
	}
	return quantize(d, spec.Scale, spec.RoundingMode)
}

func quantize(d decimal.Decimal, scale int, mode RoundingMode) decimal.Decimal {
	if scale < 0 {
		return d
	}
	switch mode {
	case RoundCeil:
		return d.RoundCeil(int32(scale))
	case RoundFloor:
		return d.RoundFloor(int32(scale))
	case RoundTruncate:
		return d.Truncate(int32(scale))
	case RoundHalfEven:
		return d.RoundBank(int32(scale))
	default:
		return d.Round(int32(scale))
	}
}

// RoundToScale rounds with an explicit mode (for formula ROUND/CEIL/FLOOR).
func RoundToScale(d decimal.Decimal, scale int, mode RoundingMode) decimal.Decimal {
	return quantize(d, scale, mode)
}

// Add, Sub, Mul, Div perform decimal arithmetic.
func Add(a, b decimal.Decimal) decimal.Decimal { return a.Add(b) }
func Sub(a, b decimal.Decimal) decimal.Decimal { return a.Sub(b) }
func Mul(a, b decimal.Decimal) decimal.Decimal { return a.Mul(b) }

func Div(a, b decimal.Decimal) (decimal.Decimal, error) {
	if b.IsZero() {
		return decimal.Zero, fmt.Errorf("division by zero")
	}
	return a.Div(b), nil
}

// Pow raises a to the power b.
func Pow(a, b decimal.Decimal) decimal.Decimal {
	af, _ := a.Float64()
	bf, _ := b.Float64()
	return decimal.NewFromFloat(math.Pow(af, bf))
}

// NormalizeAny parses, optionally rounds per spec, returns float64 for JSON storage.
func NormalizeAny(v any, spec Spec) (float64, bool) {
	d, ok := FromAny(v)
	if !ok {
		return 0, false
	}
	if spec.FinancialMode {
		d = Round(d, spec)
	}
	return ToFloat64(d), true
}

// Abs returns absolute value.
func Abs(d decimal.Decimal) decimal.Decimal { return d.Abs() }

// Compare returns -1, 0, 1.
func Compare(a, b decimal.Decimal) int { return a.Cmp(b) }

// FormatDecimal stringifies for display without float drift when scale is set.
func FormatDecimal(d decimal.Decimal, scale int) string {
	if scale >= 0 {
		return d.StringFixed(int32(scale))
	}
	s := d.String()
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if f == math.Trunc(f) && math.Abs(f) < 1e15 {
			return strconv.FormatInt(int64(f), 10)
		}
	}
	return s
}
