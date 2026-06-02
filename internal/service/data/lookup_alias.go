package data

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
)

// joinAliasRegistry assigns unique SQL table aliases and deduplicates JOIN clauses.
type joinAliasRegistry struct {
	used        map[string]struct{}
	emittedJoin map[string]struct{}
	relRowAlias map[string]string // relationship column name → shared related-table row alias
	hopAlias    map[string]string // fromRowAlias>schema.table → nested join alias
}

func newJoinAliasRegistry() *joinAliasRegistry {
	return &joinAliasRegistry{
		used:        make(map[string]struct{}),
		emittedJoin: make(map[string]struct{}),
		relRowAlias: make(map[string]string),
		hopAlias:    make(map[string]string),
	}
}

func (r *joinAliasRegistry) reserve(alias string) string {
	base := sanitizeSQLAlias(alias)
	if base == "" {
		base = "lk"
	}
	name := base
	for i := 0; ; i++ {
		if _, taken := r.used[name]; !taken {
			r.used[name] = struct{}{}
			return name
		}
		name = fmt.Sprintf("%s_%d", base, i+1)
	}
}

// sharedRelRowAlias returns one JOIN alias per relationship column (all lookups via same rel share it).
func (r *joinAliasRegistry) sharedRelRowAlias(relationshipColumnName string) string {
	if a, ok := r.relRowAlias[relationshipColumnName]; ok {
		return a
	}
	a := r.reserve("lk_rel_" + relationshipColumnName)
	r.relRowAlias[relationshipColumnName] = a
	return a
}

// ensureHopJoin returns the alias for a nested hop (from row → related table).
// joinSQL is returned only the first time this hop is needed; later callers reuse the alias.
func (r *joinAliasRegistry) ensureHopJoin(rowAlias, schema, table, lookupColName string, buildSQL func(joinAlias string) string) (joinAlias, joinSQL string) {
	key := rowAlias + ">" + schema + "." + table
	if a, ok := r.hopAlias[key]; ok {
		return a, ""
	}
	a := r.reserve(rowAlias + "_n_" + lookupColName)
	r.hopAlias[key] = a
	return a, r.appendJoin(buildSQL(a))
}

// appendJoin returns joinSQL the first time it is seen, or "" if an identical JOIN was already emitted.
func (r *joinAliasRegistry) appendJoin(joinSQL string) string {
	joinSQL = strings.TrimSpace(joinSQL)
	if joinSQL == "" {
		return ""
	}
	key := strings.Join(strings.Fields(joinSQL), " ")
	if _, ok := r.emittedJoin[key]; ok {
		return ""
	}
	r.emittedJoin[key] = struct{}{}
	return " " + joinSQL
}

func sanitizeSQLAlias(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '-' || r == '.':
			b.WriteByte('_')
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) > 55 {
		out = out[:55]
	}
	return out
}

func quotedAlias(alias string) string {
	return pgx.Identifier{alias}.Sanitize()
}

// hostRelJoinKey identifies the base LEFT JOIN to a related table from the query base row.
func hostRelJoinKey(schema, table, baseFKPgCol string) string {
	return schema + "." + table + "|" + baseFKPgCol
}
