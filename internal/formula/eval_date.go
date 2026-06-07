package formula

import (
	"fmt"
	"strings"
	"time"
)

// nowFn is the clock used by TODAY (overridable in tests).
var nowFn = time.Now

func evalTODAY(c CallNode) (any, error) {
	if len(c.Args) != 0 {
		return nil, fmt.Errorf("formula: TODAY takes no arguments")
	}
	return dateOnly(nowFn()), nil
}

func evalNOW(c CallNode) (any, error) {
	if len(c.Args) != 0 {
		return nil, fmt.Errorf("formula: NOW takes no arguments")
	}
	return nowFn(), nil
}

func evalDATEVALUE(c CallNode, env map[string]any) (any, error) {
	if len(c.Args) != 1 {
		return nil, fmt.Errorf("formula: DATEVALUE needs 1 argument")
	}
	v, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	t, ok := asTime(v)
	if !ok {
		return nil, fmt.Errorf("formula: DATEVALUE needs a date text")
	}
	return dateOnly(t), nil
}

func evalWEEKDAY(c CallNode, env map[string]any) (any, error) {
	if len(c.Args) < 1 || len(c.Args) > 2 {
		return nil, fmt.Errorf("formula: WEEKDAY needs 1 or 2 arguments")
	}
	v, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	t, ok := asTime(v)
	if !ok {
		return nil, fmt.Errorf("formula: WEEKDAY needs a date")
	}
	ret := 1
	if len(c.Args) == 2 {
		r, err := Eval(c.Args[1], env)
		if err != nil {
			return nil, err
		}
		n, ok := truncInt(r)
		if !ok {
			return nil, fmt.Errorf("formula: WEEKDAY return_type needs a number")
		}
		ret = n
	}
	n, err := weekdayExcel(t, ret)
	if err != nil {
		return nil, err
	}
	return float64(n), nil
}

func weekdayExcel(t time.Time, returnType int) (int, error) {
	wd := int(t.Weekday()) // Sunday=0 ... Saturday=6
	switch returnType {
	case 1:
		return wd + 1, nil
	case 2:
		if wd == 0 {
			return 7, nil
		}
		return wd, nil
	case 3:
		return (wd + 6) % 7, nil
	case 11, 12, 13, 14, 15, 16, 17:
		start := returnType - 11 // 0=Monday ... 6=Sunday
		mon0 := (wd + 6) % 7     // Monday=0 ... Sunday=6
		return (mon0-start+7)%7 + 1, nil
	default:
		return 0, fmt.Errorf("formula: WEEKDAY return_type must be 1, 2, 3, or 11-17")
	}
}

func evalDATE(c CallNode, env map[string]any) (any, error) {
	if len(c.Args) != 3 {
		return nil, fmt.Errorf("formula: DATE needs 3 arguments")
	}
	yV, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	mV, err := Eval(c.Args[1], env)
	if err != nil {
		return nil, err
	}
	dV, err := Eval(c.Args[2], env)
	if err != nil {
		return nil, err
	}
	y, okY := truncInt(yV)
	m, okM := truncInt(mV)
	d, okD := truncInt(dV)
	if !okY || !okM || !okD {
		return nil, fmt.Errorf("formula: DATE needs numbers")
	}
	if y < 0 {
		return nil, fmt.Errorf("formula: DATE year must be >= 0")
	}
	if y <= 1899 {
		y += 1900
	}
	if y > 9999 {
		return nil, fmt.Errorf("formula: DATE year must be <= 9999")
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local), nil
}

func evalDatePart(c CallNode, env map[string]any, which string) (any, error) {
	if len(c.Args) != 1 {
		return nil, fmt.Errorf("formula: %s needs 1 argument", which)
	}
	v, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	t, ok := asTime(v)
	if !ok {
		return nil, fmt.Errorf("formula: %s needs a date", which)
	}
	switch which {
	case "YEAR":
		return float64(t.Year()), nil
	case "MONTH":
		return float64(t.Month()), nil
	default:
		return float64(t.Day()), nil
	}
}

func evalDATEDIF(c CallNode, env map[string]any) (any, error) {
	if len(c.Args) != 3 {
		return nil, fmt.Errorf("formula: DATEDIF needs 3 arguments")
	}
	sV, err := Eval(c.Args[0], env)
	if err != nil {
		return nil, err
	}
	eV, err := Eval(c.Args[1], env)
	if err != nil {
		return nil, err
	}
	uV, err := Eval(c.Args[2], env)
	if err != nil {
		return nil, err
	}
	start, okS := asTime(sV)
	end, okE := asTime(eV)
	if !okS || !okE {
		return nil, fmt.Errorf("formula: DATEDIF needs dates")
	}
	start, end = dateOnly(start), dateOnly(end)
	if end.Before(start) {
		return nil, fmt.Errorf("formula: DATEDIF end must be >= start")
	}
	unit := strings.ToUpper(strings.TrimSpace(toString(uV)))
	n, err := datedif(start, end, unit)
	if err != nil {
		return nil, err
	}
	return float64(n), nil
}

func datedif(start, end time.Time, unit string) (int, error) {
	sy, sm, sd := start.Date()
	ey, em, ed := end.Date()
	switch unit {
	case "Y":
		y := ey - sy
		if em < sm || (em == sm && ed < sd) {
			y--
		}
		return y, nil
	case "M":
		m := (ey-sy)*12 + int(em-sm)
		if ed < sd {
			m--
		}
		return m, nil
	case "D":
		return daysBetween(start, end), nil
	case "YM":
		m := int(em - sm)
		if ed < sd {
			m--
		}
		if m < 0 {
			m += 12
		}
		return m, nil
	case "MD":
		if ed >= sd {
			return ed - sd, nil
		}
		prev := time.Date(ey, em, 0, 0, 0, 0, 0, time.UTC)
		return prev.Day() - sd + ed, nil
	case "YD":
		ann := time.Date(ey, sm, sd, 0, 0, 0, 0, time.UTC)
		endU := time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC)
		if ann.After(endU) {
			ann = time.Date(ey-1, sm, sd, 0, 0, 0, 0, time.UTC)
		}
		return daysBetween(ann, endU), nil
	default:
		return 0, fmt.Errorf("formula: DATEDIF unit must be Y, M, D, YM, MD, or YD")
	}
}

func daysBetween(start, end time.Time) int {
	a := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a) / (24 * time.Hour))
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func asTime(v any) (time.Time, bool) {
	if v == nil {
		return time.Time{}, false
	}
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		return parseDateString(strings.TrimSpace(t))
	default:
		return time.Time{}, false
	}
}

func parseDateString(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006/01/02",
		"2006/01/02 15:04:05",
		"2006.01.02",
		"20060102",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	if t, ok := parseLooseDate(s); ok {
		return t, true
	}
	return time.Time{}, false
}

func parseLooseDate(s string) (time.Time, bool) {
	datePart := s
	if i := strings.IndexAny(s, "T "); i > 0 {
		datePart = s[:i]
	}
	datePart = strings.ReplaceAll(datePart, ".", "-")
	datePart = strings.ReplaceAll(datePart, "/", "-")
	parts := strings.Split(datePart, "-")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	y, okY := truncInt(parts[0])
	m, okM := truncInt(parts[1])
	d, okD := truncInt(parts[2])
	if !okY || !okM || !okD || y < 1 || m < 1 || m > 12 || d < 1 || d > 31 {
		return time.Time{}, false
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local), true
}
