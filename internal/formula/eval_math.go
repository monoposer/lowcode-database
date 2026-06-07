package formula

import (
	"fmt"
	"math"
)

func evalMinMax(c CallNode, env map[string]any, min bool) (any, error) {
	name := "MAX"
	if min {
		name = "MIN"
	}
	if len(c.Args) < 1 {
		return nil, fmt.Errorf("formula: %s needs at least 1 argument", name)
	}
	var best float64
	for i, a := range c.Args {
		v, err := Eval(a, env)
		if err != nil {
			return nil, err
		}
		n, ok := toFloat(v)
		if !ok {
			return nil, fmt.Errorf("formula: %s needs numbers", name)
		}
		if i == 0 {
			best = n
			continue
		}
		if min {
			best = math.Min(best, n)
		} else {
			best = math.Max(best, n)
		}
	}
	return best, nil
}

func evalINT(c CallNode, env map[string]any) (any, error) {
	if len(c.Args) != 1 {
		return nil, fmt.Errorf("formula: INT needs 1 argument")
	}
	v, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	n, ok := toFloat(v)
	if !ok {
		return nil, fmt.Errorf("formula: INT needs a number")
	}
	return math.Floor(n), nil
}

func evalISBLANK(c CallNode, env map[string]any) (any, error) {
	if len(c.Args) != 1 {
		return nil, fmt.Errorf("formula: ISBLANK needs 1 argument")
	}
	v, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	return isBlank(v), nil
}

func isBlank(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}
