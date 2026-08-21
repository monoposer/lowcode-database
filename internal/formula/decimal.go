package formula

import (
	"fmt"

	"github.com/monoposer/lowcode-database/internal/numeric"
	"github.com/shopspring/decimal"
)

func toDecimal(v any) (decimal.Decimal, bool) {
	return numeric.FromAny(v)
}

func decimalToResult(d decimal.Decimal) float64 {
	return numeric.ToFloat64(d)
}

func evalArithDecimal(op string, l, r any) (any, error) {
	a, okA := toDecimal(l)
	b, okB := toDecimal(r)
	if !okA || !okB {
		if op == "+" {
			return toString(l) + toString(r), nil
		}
		return nil, fmt.Errorf("formula: %s needs numbers", op)
	}
	switch op {
	case "+":
		return decimalToResult(numeric.Add(a, b)), nil
	case "-":
		return decimalToResult(numeric.Sub(a, b)), nil
	case "*":
		return decimalToResult(numeric.Mul(a, b)), nil
	case "/":
		q, err := numeric.Div(a, b)
		if err != nil {
			return nil, fmt.Errorf("formula: division by zero")
		}
		return decimalToResult(q), nil
	case "^":
		return decimalToResult(numeric.Pow(a, b)), nil
	default:
		return nil, fmt.Errorf("formula: unknown operator %q", op)
	}
}

func evalCompareDecimal(op string, l, r any) (bool, error) {
	lf, okL := toDecimal(l)
	rf, okR := toDecimal(r)
	if !okL || !okR {
		return false, nil
	}
	cmp := numeric.Compare(lf, rf)
	switch op {
	case "=":
		return cmp == 0, nil
	case "<>":
		return cmp != 0, nil
	case "<":
		return cmp < 0, nil
	case ">":
		return cmp > 0, nil
	case "<=":
		return cmp <= 0, nil
	case ">=":
		return cmp >= 0, nil
	default:
		return false, fmt.Errorf("formula: unknown compare %q", op)
	}
}

func evalRoundCall(c CallNode, env map[string]any, mode numeric.RoundingMode) (any, error) {
	if len(c.Args) < 1 || len(c.Args) > 2 {
		return nil, fmt.Errorf("formula: ROUND needs 1 or 2 arguments")
	}
	v, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	d, ok := toDecimal(v)
	if !ok {
		return nil, fmt.Errorf("formula: ROUND needs a number")
	}
	scale := 0
	if len(c.Args) == 2 {
		digits, err := Eval(c.Args[1], env)
		if err != nil {
			return nil, err
		}
		if n, ok := toDecimal(digits); ok {
			scale = int(n.IntPart())
		}
	}
	return decimalToResult(numeric.RoundToScale(d, scale, mode)), nil
}

func evalCeilFloorCall(c CallNode, env map[string]any, ceil bool) (any, error) {
	name := "FLOOR"
	mode := numeric.RoundFloor
	if ceil {
		name = "CEIL"
		mode = numeric.RoundCeil
	}
	if len(c.Args) < 1 || len(c.Args) > 2 {
		return nil, fmt.Errorf("formula: %s needs 1 or 2 arguments", name)
	}
	v, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	d, ok := toDecimal(v)
	if !ok {
		return nil, fmt.Errorf("formula: %s needs a number", name)
	}
	scale := 0
	if len(c.Args) == 2 {
		digits, err := Eval(c.Args[1], env)
		if err != nil {
			return nil, err
		}
		if n, ok := toDecimal(digits); ok {
			scale = int(n.IntPart())
		}
	}
	return decimalToResult(numeric.RoundToScale(d, scale, mode)), nil
}

func evalSumDecimal(c CallNode, env map[string]any) (any, error) {
	sum := decimal.Zero
	for _, a := range c.Args {
		v, err := Eval(a, env)
		if err != nil {
			return nil, err
		}
		n, ok := toDecimal(v)
		if !ok {
			return nil, fmt.Errorf("formula: SUM needs numbers")
		}
		sum = numeric.Add(sum, n)
	}
	return decimalToResult(sum), nil
}

func evalAbsDecimal(v any) (any, error) {
	n, ok := toDecimal(v)
	if !ok {
		return nil, fmt.Errorf("formula: ABS needs a number")
	}
	return decimalToResult(numeric.Abs(n)), nil
}

func evalMinMaxDecimal(c CallNode, env map[string]any, min bool) (any, error) {
	name := "MAX"
	if min {
		name = "MIN"
	}
	if len(c.Args) < 1 {
		return nil, fmt.Errorf("formula: %s needs at least 1 argument", name)
	}
	var best decimal.Decimal
	for i, a := range c.Args {
		v, err := Eval(a, env)
		if err != nil {
			return nil, err
		}
		n, ok := toDecimal(v)
		if !ok {
			return nil, fmt.Errorf("formula: %s needs numbers", name)
		}
		if i == 0 {
			best = n
			continue
		}
		cmp := numeric.Compare(n, best)
		if min {
			if cmp < 0 {
				best = n
			}
		} else if cmp > 0 {
			best = n
		}
	}
	return decimalToResult(best), nil
}

func evalINTDecimal(v any) (any, error) {
	n, ok := toDecimal(v)
	if !ok {
		return nil, fmt.Errorf("formula: INT needs a number")
	}
	return decimalToResult(numeric.RoundToScale(n, 0, numeric.RoundFloor)), nil
}
