package formula

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

func evalMID(c CallNode, env map[string]any) (any, error) {
	if len(c.Args) != 3 {
		return nil, fmt.Errorf("formula: MID needs 3 arguments")
	}
	textV, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	startV, err := Eval(c.Args[1], env)
	if err != nil {
		return nil, err
	}
	nV, err := Eval(c.Args[2], env)
	if err != nil {
		return nil, err
	}
	start, ok := truncInt(startV)
	if !ok || start < 1 {
		return nil, fmt.Errorf("formula: MID start_num must be >= 1")
	}
	n, ok := truncInt(nV)
	if !ok || n < 0 {
		return nil, fmt.Errorf("formula: MID num_chars must be >= 0")
	}
	runes := []rune(toString(textV))
	if n == 0 || start > len(runes) {
		return "", nil
	}
	i := start - 1
	end := i + n
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[i:end]), nil
}

func evalTRIM(c CallNode, env map[string]any) (any, error) {
	if len(c.Args) != 1 {
		return nil, fmt.Errorf("formula: TRIM needs 1 argument")
	}
	v, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	return excelTrim(toString(v)), nil
}

func evalCase(c CallNode, env map[string]any, which string) (any, error) {
	if len(c.Args) != 1 {
		return nil, fmt.Errorf("formula: %s needs 1 argument", which)
	}
	v, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	s := toString(v)
	if which == "UPPER" {
		return strings.ToUpper(s), nil
	}
	return strings.ToLower(s), nil
}

func excelTrim(s string) string {
	var b strings.Builder
	pending := false
	started := false
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			if started {
				pending = true
			}
			continue
		}
		if pending {
			b.WriteByte(' ')
			pending = false
		}
		started = true
		b.WriteByte(s[i])
	}
	return b.String()
}

func evalLEN(c CallNode, env map[string]any) (any, error) {
	if len(c.Args) != 1 {
		return nil, fmt.Errorf("formula: LEN needs 1 argument")
	}
	v, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	return float64(utf8.RuneCountInString(toString(v))), nil
}

func truncInt(v any) (int, bool) {
	n, ok := toFloat(v)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return int(math.Trunc(n)), true
}
