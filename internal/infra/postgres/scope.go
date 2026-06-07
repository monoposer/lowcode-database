package postgres

import (
	"fmt"
	"regexp"
	"strings"
)

var identName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Eq is a trusted column = value predicate. Empty Val is omitted
// (worker ticks have no tenant in context).
type Eq struct {
	Col string
	Val string
}

// Where joins non-empty Eqs as `col = $n AND …`. Col must be a SQL identifier constant.
func Where(start int, eqs ...Eq) (clause string, args []any, next int) {
	n := start
	var parts []string
	for _, e := range eqs {
		if strings.TrimSpace(e.Val) == "" {
			continue
		}
		if !identName.MatchString(e.Col) {
			panic("postgres.Where: invalid identifier " + e.Col)
		}
		parts = append(parts, fmt.Sprintf("%s = $%d", e.Col, n))
		args = append(args, e.Val)
		n++
	}
	return strings.Join(parts, " AND "), args, n
}

// AndWhere appends ` AND …` from Where, or returns sql unchanged if every Val is empty.
func AndWhere(sql string, args []any, start int, eqs ...Eq) (string, []any, int) {
	clause, extra, next := Where(start, eqs...)
	if clause == "" {
		return sql, args, start
	}
	return sql + " AND " + clause, append(args, extra...), next
}

// TenantBase is the usual meta-table tenant scope.
func TenantBase(tenantID, baseID string) []Eq {
	return []Eq{{Col: "tenant_id", Val: tenantID}, {Col: "base_id", Val: baseID}}
}
