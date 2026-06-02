// Package query builds SQL WHERE/ORDER BY from DSL and column metadata.
package query

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/dsl"
)

// ColumnMeta is minimal column info for SQL generation.
type ColumnMeta struct {
	ID     string
	Name   string // PG column name (= logical name)
	PgType string
}

// OrderSpec is a sort directive.
type OrderSpec struct {
	Attribute string
	SortOrder string // ASC | DESC
}

// BuildWhere renders a WHERE clause (without the WHERE keyword) and args starting at argStart.
func BuildWhere(w dsl.Where, attrToPg map[string]string, argStart int) (string, []any, error) {
	return BuildWhereWithTypes(w, attrToPg, nil, argStart)
}

// BuildWhereWithTypes is like BuildWhere but uses attrPgTypes for array-aware operators.
func BuildWhereWithTypes(w dsl.Where, attrToPg, attrPgTypes map[string]string, argStart int) (string, []any, error) {
	if w.Type == "" {
		return "", nil, nil
	}
	switch w.Type {
	case "AND", "OR":
		var parts []string
		var args []any
		idx := argStart
		for _, child := range w.Vals {
			part, childArgs, err := BuildWhereWithTypes(child, attrToPg, attrPgTypes, idx)
			if err != nil {
				return "", nil, err
			}
			if part == "" {
				continue
			}
			parts = append(parts, "("+part+")")
			args = append(args, childArgs...)
			idx += len(childArgs)
		}
		if len(parts) == 0 {
			return "", nil, nil
		}
		join := " AND "
		if w.Type == "OR" {
			join = " OR "
		}
		return strings.Join(parts, join), args, nil
	case "LIKE":
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		if isArrayPgType(pgType) {
			return buildArrayHas(colRef, pgType, w.Val, argStart)
		}
		return colRef + " LIKE " + fmt.Sprintf("$%d", argStart), []any{likeContainsPattern(w.Val)}, nil
	case "ARRAY_HAS":
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		return buildArrayHas(colRef, pgType, w.Val, argStart)
	case "ARRAY_NOT_HAS":
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		return buildArrayNotHas(colRef, pgType, w.Val, argStart)
	case "ARRAY_OVERLAP":
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		return buildArrayOverlap(colRef, pgType, w.Val, argStart)
	case "ARRAY_NOT_OVERLAP":
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		return buildArrayNotOverlap(colRef, pgType, w.Val, argStart)
	case "ARRAY_CONTAINS":
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		return buildArrayContains(colRef, pgType, w.Val, argStart)
	case "ARRAY_NOT_CONTAINS":
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		return buildArrayNotContains(colRef, pgType, w.Val, argStart)
	case "EQ", "NEQ", "GT", "GTE", "LT", "LTE":
		colRef, _, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		op := map[string]string{
			"EQ": " = ", "NEQ": " <> ", "GT": " > ", "GTE": " >= ", "LT": " < ", "LTE": " <= ",
		}[w.Type]
		return colRef + op + fmt.Sprintf("$%d", argStart), []any{w.Val}, nil
	case "IN", "NIN":
		colRef, _, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		vals, ok := w.Val.([]any)
		if !ok {
			if arr, ok2 := w.Val.([]interface{}); ok2 {
				vals = arr
			} else {
				return "", nil, fmt.Errorf("IN filter val must be array")
			}
		}
		if len(vals) == 0 {
			if w.Type == "IN" {
				return "FALSE", nil, nil
			}
			return "TRUE", nil, nil
		}
		placeholders := make([]string, len(vals))
		args := make([]any, len(vals))
		for i, v := range vals {
			placeholders[i] = fmt.Sprintf("$%d", argStart+i)
			args[i] = v
		}
		op := " IN "
		if w.Type == "NIN" {
			op = " NOT IN "
		}
		return colRef + op + "(" + strings.Join(placeholders, ", ") + ")", args, nil
	case "EMPTY":
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		if isArrayPgType(pgType) {
			cast := arrayCastType(pgType)
			return fmt.Sprintf("(%s IS NULL OR %s = '{}'::%s)", colRef, colRef, cast), nil, nil
		}
		return colRef + " IS NULL", nil, nil
	case "NOT_EMPTY":
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		if isArrayPgType(pgType) {
			cast := arrayCastType(pgType)
			return fmt.Sprintf("(%s IS NOT NULL AND %s <> '{}'::%s)", colRef, colRef, cast), nil, nil
		}
		return colRef + " IS NOT NULL", nil, nil
	default:
		return "", nil, fmt.Errorf("unsupported filter type %q", w.Type)
	}
}

func resolveColRefWithType(attr string, attrToPg, attrPgTypes map[string]string) (colRef string, pgType string, err error) {
	pg, ok := attrToPg[attr]
	if !ok {
		return "", "", fmt.Errorf("unknown filter attribute %q", attr)
	}
	colRef = pg
	if !strings.Contains(pg, ".") {
		colRef = pgx.Identifier{pg}.Sanitize()
	}
	if attrPgTypes != nil {
		pgType = attrPgTypes[attr]
	}
	return colRef, pgType, nil
}

func isArrayPgType(pgType string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(pgType)), "[]")
}

func arrayCastType(pgType string) string {
	if isArrayPgType(pgType) {
		return pgType
	}
	return "text[]"
}

func filterValToSlice(val any) ([]any, error) {
	switch v := val.(type) {
	case []any:
		if len(v) == 0 {
			return nil, fmt.Errorf("array filter val must be non-empty")
		}
		return v, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("array filter val must be non-empty")
		}
		return []any{v}, nil
	default:
		if val == nil {
			return nil, fmt.Errorf("array filter val must be non-empty")
		}
		return []any{val}, nil
	}
}

func isArrayValueRef(colRef, pgType string) bool {
	if isArrayPgType(pgType) {
		return true
	}
	// Virtual many-lookup columns are scalar subqueries returning text[] (or other arrays).
	return strings.HasPrefix(strings.TrimSpace(colRef), "(")
}

func buildArrayHas(colRef, pgType string, val any, argStart int) (string, []any, error) {
	s, ok := val.(string)
	if !ok {
		s = fmt.Sprint(val)
	}
	if strings.TrimSpace(s) == "" {
		return "", nil, fmt.Errorf("ARRAY_HAS filter val must be non-empty")
	}
	if isArrayValueRef(colRef, pgType) {
		cast := arrayCastType(pgType)
		return fmt.Sprintf("(%s) @> ARRAY[$%d]::%s", colRef, argStart, cast), []any{s}, nil
	}
	return fmt.Sprintf("$%d = ANY(%s)", argStart, colRef), []any{s}, nil
}

func buildArrayNotHas(colRef, pgType string, val any, argStart int) (string, []any, error) {
	sql, args, err := buildArrayHas(colRef, pgType, val, argStart)
	if err != nil {
		return "", nil, err
	}
	return "NOT (" + sql + ")", args, nil
}

func buildArrayOverlap(colRef, pgType string, val any, argStart int) (string, []any, error) {
	vals, err := filterValToSlice(val)
	if err != nil {
		return "", nil, err
	}
	cast := arrayCastType(pgType)
	return fmt.Sprintf("(%s) && $%d::%s", colRef, argStart, cast), []any{anySliceToStringSlice(vals)}, nil
}

func buildArrayNotOverlap(colRef, pgType string, val any, argStart int) (string, []any, error) {
	sql, args, err := buildArrayOverlap(colRef, pgType, val, argStart)
	if err != nil {
		return "", nil, err
	}
	return "NOT (" + sql + ")", args, nil
}

func buildArrayContains(colRef, pgType string, val any, argStart int) (string, []any, error) {
	vals, err := filterValToSlice(val)
	if err != nil {
		return "", nil, err
	}
	cast := arrayCastType(pgType)
	return fmt.Sprintf("(%s) @> $%d::%s", colRef, argStart, cast), []any{anySliceToStringSlice(vals)}, nil
}

func buildArrayNotContains(colRef, pgType string, val any, argStart int) (string, []any, error) {
	sql, args, err := buildArrayContains(colRef, pgType, val, argStart)
	if err != nil {
		return "", nil, err
	}
	return "NOT (" + sql + ")", args, nil
}

func anySliceToStringSlice(vals []any) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = fmt.Sprint(v)
	}
	return out
}

// AttrPgTypesFromColumns maps column id/name -> PostgreSQL type for array-aware filters.
func AttrPgTypesFromColumns(cols []ColumnMeta) map[string]string {
	m := make(map[string]string, len(cols)*2)
	for _, c := range cols {
		if c.PgType == "" {
			continue
		}
		if c.ID != "" {
			m[c.ID] = c.PgType
		}
		if c.Name != "" {
			m[c.Name] = c.PgType
		}
	}
	return m
}

// likeContainsPattern turns a filter value into a SQL LIKE "contains" pattern (%…%).
// Literal % and _ in the input are escaped; if the value already contains %, it is used as-is.
func likeContainsPattern(val any) any {
	s, ok := val.(string)
	if !ok {
		return val
	}
	if strings.Contains(s, "%") {
		return escapeLikeLiteral(s)
	}
	return "%" + escapeLikeLiteral(s) + "%"
}

func escapeLikeLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '%', '_':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// BuildOrderBy renders ORDER BY clause (without keyword).
func BuildOrderBy(orders []OrderSpec, attrToPg map[string]string, defaultCol string) string {
	var parts []string
	for _, o := range orders {
		pg, ok := attrToPg[o.Attribute]
		if !ok {
			continue
		}
		dir := "ASC"
		if strings.EqualFold(o.SortOrder, "DESC") {
			dir = "DESC"
		}
		ref := pg
		if !strings.Contains(pg, ".") {
			ref = pgx.Identifier{pg}.Sanitize()
		}
		parts = append(parts, ref+" "+dir)
	}
	if len(parts) == 0 && defaultCol != "" {
		ref := defaultCol
		if r, ok := attrToPg[defaultCol]; ok {
			ref = r
		} else if !strings.Contains(defaultCol, ".") {
			ref = pgx.Identifier{defaultCol}.Sanitize()
		}
		parts = append(parts, ref+" ASC")
	}
	return strings.Join(parts, ", ")
}

// AttrMapFromColumns builds attribute name/id -> qualified pg column reference.
func AttrMapFromColumns(alias string, cols []ColumnMeta) map[string]string {
	m := map[string]string{"id": alias + ".id"}
	for _, c := range cols {
		ref := alias + "." + pgx.Identifier{c.Name}.Sanitize()
		m[c.ID] = ref
		m[c.Name] = ref
	}
	return m
}
