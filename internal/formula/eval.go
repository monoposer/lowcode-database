package formula

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/monoposer/lowcode-database/internal/numeric"
	"github.com/shopspring/decimal"
)

// EvalExpr parses and evaluates a formula against column values (application layer).
func EvalExpr(expr string, env map[string]any) (any, error) {
	n, err := Parse(expr)
	if err != nil {
		return nil, err
	}
	return Eval(n, env)
}

// Eval walks an efp-derived AST.
func Eval(n Node, env map[string]any) (any, error) {
	if n == nil {
		return nil, nil
	}
	switch t := n.(type) {
	case NumberNode:
		return t.Value, nil
	case StringNode:
		return t.Value, nil
	case BoolNode:
		return t.Value, nil
	case RefNode:
		if env == nil {
			return nil, nil
		}
		return env[t.Name], nil
	case UnaryNode:
		x, err := Eval(t.X, env)
		if err != nil {
			return nil, err
		}
		return evalUnary(t.Op, x)
	case BinaryNode:
		if t.Op == "=" || t.Op == "<>" || t.Op == "<" || t.Op == ">" || t.Op == "<=" || t.Op == ">=" {
			l, err := Eval(t.L, env)
			if err != nil {
				return nil, err
			}
			r, err := Eval(t.R, env)
			if err != nil {
				return nil, err
			}
			return evalCompare(t.Op, l, r)
		}
		if t.Op == "&" {
			l, err := Eval(t.L, env)
			if err != nil {
				return nil, err
			}
			r, err := Eval(t.R, env)
			if err != nil {
				return nil, err
			}
			return toString(l) + toString(r), nil
		}
		l, err := Eval(t.L, env)
		if err != nil {
			return nil, err
		}
		r, err := Eval(t.R, env)
		if err != nil {
			return nil, err
		}
		return evalArithDecimal(t.Op, l, r)
	case CallNode:
		return evalCall(t, env)
	default:
		return nil, fmt.Errorf("formula: unknown node")
	}
}

func evalUnary(op string, x any) (any, error) {
	switch op {
	case "-":
		n, ok := toDecimal(x)
		if !ok {
			return nil, fmt.Errorf("formula: unary - needs a number")
		}
		return decimalToResult(n.Neg()), nil
	case "+":
		n, ok := toDecimal(x)
		if !ok {
			return nil, fmt.Errorf("formula: unary + needs a number")
		}
		return decimalToResult(n), nil
	case "%":
		n, ok := toDecimal(x)
		if !ok {
			return nil, fmt.Errorf("formula: %% needs a number")
		}
		q, err := numeric.Div(n, decimal.NewFromInt(100))
		if err != nil {
			return nil, err
		}
		return decimalToResult(q), nil
	default:
		return nil, fmt.Errorf("formula: unknown unary %q", op)
	}
}

func evalArith(op string, l, r any) (any, error) {
	a, okA := toFloat(l)
	b, okB := toFloat(r)
	if !okA || !okB {
		if op == "+" {
			return toString(l) + toString(r), nil
		}
		return nil, fmt.Errorf("formula: %s needs numbers", op)
	}
	switch op {
	case "+":
		return a + b, nil
	case "-":
		return a - b, nil
	case "*":
		return a * b, nil
	case "/":
		if b == 0 {
			return nil, fmt.Errorf("formula: division by zero")
		}
		return a / b, nil
	case "^":
		return math.Pow(a, b), nil
	default:
		return nil, fmt.Errorf("formula: unknown operator %q", op)
	}
}

func evalCompare(op string, l, r any) (bool, error) {
	if lt, okL := asTime(l); okL {
		if rt, okR := asTime(r); okR {
			switch op {
			case "=":
				return lt.Equal(rt), nil
			case "<>":
				return !lt.Equal(rt), nil
			case "<":
				return lt.Before(rt), nil
			case ">":
				return lt.After(rt), nil
			case "<=":
				return lt.Before(rt) || lt.Equal(rt), nil
			case ">=":
				return lt.After(rt) || lt.Equal(rt), nil
			}
		}
	}
	if _, okL := toDecimal(l); okL {
		if _, okR := toDecimal(r); okR {
			return evalCompareDecimal(op, l, r)
		}
	}
	if lf, okL := toFloat(l); okL {
		if rf, okR := toFloat(r); okR {
			switch op {
			case "=":
				return lf == rf, nil
			case "<>":
				return lf != rf, nil
			case "<":
				return lf < rf, nil
			case ">":
				return lf > rf, nil
			case "<=":
				return lf <= rf, nil
			case ">=":
				return lf >= rf, nil
			}
		}
	}
	ls, rs := toString(l), toString(r)
	switch op {
	case "=":
		return ls == rs, nil
	case "<>":
		return ls != rs, nil
	case "<":
		return ls < rs, nil
	case ">":
		return ls > rs, nil
	case "<=":
		return ls <= rs, nil
	case ">=":
		return ls >= rs, nil
	default:
		return false, fmt.Errorf("formula: unknown compare %q", op)
	}
}

func evalCall(c CallNode, env map[string]any) (any, error) {
	switch c.Name {
	case "IF":
		if len(c.Args) < 2 || len(c.Args) > 3 {
			return nil, fmt.Errorf("formula: IF needs 2 or 3 arguments")
		}
		cond, err := Eval(c.Args[0], env)
		if err != nil {
			return nil, err
		}
		if truthy(cond) {
			return Eval(c.Args[1], env)
		}
		if len(c.Args) == 3 {
			return Eval(c.Args[2], env)
		}
		return false, nil
	case "SUM":
		return evalSumDecimal(c, env)
	case "CONCAT", "CONCATENATE":
		var b strings.Builder
		for _, a := range c.Args {
			v, err := Eval(a, env)
			if err != nil {
				return nil, err
			}
			b.WriteString(toString(v))
		}
		return b.String(), nil
	case "AND":
		for _, a := range c.Args {
			v, err := Eval(a, env)
			if err != nil {
				return nil, err
			}
			if !truthy(v) {
				return false, nil
			}
		}
		return true, nil
	case "OR":
		for _, a := range c.Args {
			v, err := Eval(a, env)
			if err != nil {
				return nil, err
			}
			if truthy(v) {
				return true, nil
			}
		}
		return false, nil
	case "NOT":
		if len(c.Args) != 1 {
			return nil, fmt.Errorf("formula: NOT needs 1 argument")
		}
		v, err := Eval(c.Args[0], env)
		if err != nil {
			return nil, err
		}
		return !truthy(v), nil
	case "TRUE":
		return true, nil
	case "FALSE":
		return false, nil
	case "TEXT":
		if len(c.Args) < 1 {
			return nil, fmt.Errorf("formula: TEXT needs an argument")
		}
		v, err := Eval(c.Args[0], env)
		if err != nil {
			return nil, err
		}
		return toString(v), nil
	case "ABS":
		if len(c.Args) != 1 {
			return nil, fmt.Errorf("formula: ABS needs 1 argument")
		}
		v, err := Eval(c.Args[0], env)
		if err != nil {
			return nil, err
		}
		return evalAbsDecimal(v)
	case "ROUND":
		return evalRoundCall(c, env, numeric.RoundHalfUp)
	case "CEIL", "CEILING":
		return evalCeilFloorCall(c, env, true)
	case "FLOOR":
		return evalCeilFloorCall(c, env, false)
	case "MIN":
		return evalMinMaxDecimal(c, env, true)
	case "MAX":
		return evalMinMaxDecimal(c, env, false)
	case "INT":
		if len(c.Args) != 1 {
			return nil, fmt.Errorf("formula: INT needs 1 argument")
		}
		v, err := Eval(c.Args[0], env)
		if err != nil {
			return nil, err
		}
		return evalINTDecimal(v)
	case "ISBLANK":
		return evalISBLANK(c, env)
	case "TRIM":
		return evalTRIM(c, env)
	case "UPPER":
		return evalCase(c, env, "UPPER")
	case "LOWER":
		return evalCase(c, env, "LOWER")
	case "MID":
		return evalMID(c, env)
	case "LEN":
		return evalLEN(c, env)
	case "DATE":
		return evalDATE(c, env)
	case "TODAY":
		return evalTODAY(c)
	case "NOW":
		return evalNOW(c)
	case "DATEVALUE":
		return evalDATEVALUE(c, env)
	case "WEEKDAY":
		return evalWEEKDAY(c, env)
	case "YEAR":
		return evalDatePart(c, env, "YEAR")
	case "MONTH":
		return evalDatePart(c, env, "MONTH")
	case "DAY":
		return evalDatePart(c, env, "DAY")
	case "DATEDIF":
		return evalDATEDIF(c, env)
	default:
		return nil, fmt.Errorf("formula: unsupported function %s", c.Name)
	}
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case nil:
		return 0, false
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	case bool:
		if t {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case time.Time:
		if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
			return t.Format("2006-01-02")
		}
		return t.Format(time.RFC3339)
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	case float64:
		if t == math.Trunc(t) && math.Abs(t) < 1e15 {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		if n, ok := toFloat(v); ok {
			if n == math.Trunc(n) && math.Abs(n) < 1e15 {
				return strconv.FormatInt(int64(n), 10)
			}
			return strconv.FormatFloat(n, 'f', -1, 64)
		}
		return fmt.Sprint(v)
	}
}

func truthy(v any) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t != ""
	default:
		n, ok := toFloat(v)
		if ok {
			return n != 0
		}
		return true
	}
}
