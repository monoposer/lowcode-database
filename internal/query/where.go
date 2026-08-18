package query

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/dsl"
)

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
		return buildWhereLogical(w, attrToPg, attrPgTypes, argStart)
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
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		if isArrayPgType(pgType) && (w.Type == "EQ" || w.Type == "NEQ") {
			if w.Type == "EQ" {
				return buildArrayHas(colRef, pgType, w.Val, argStart)
			}
			return buildArrayNotHas(colRef, pgType, w.Val, argStart)
		}
		return buildWhereCompare(w, attrToPg, attrPgTypes, argStart)
	case "IN", "NIN":
		colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
		if err != nil {
			return "", nil, err
		}
		if isArrayPgType(pgType) {
			if w.Type == "IN" {
				return buildArrayOverlap(colRef, pgType, w.Val, argStart)
			}
			return buildArrayNotOverlap(colRef, pgType, w.Val, argStart)
		}
		return buildWhereIn(w, attrToPg, attrPgTypes, argStart)
	case "BETWEEN":
		return buildWhereBetween(w, attrToPg, attrPgTypes, argStart)
	case "EMPTY":
		return buildWhereEmpty(w, attrToPg, attrPgTypes)
	case "NOT_EMPTY":
		return buildWhereNotEmpty(w, attrToPg, attrPgTypes)
	case "FTS":
		return buildWhereFTS(w, attrToPg, attrPgTypes, argStart)
	default:
		return "", nil, fmt.Errorf("unsupported filter type %q", w.Type)
	}
}

func buildWhereLogical(w dsl.Where, attrToPg, attrPgTypes map[string]string, argStart int) (string, []any, error) {
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
}

func buildWhereCompare(w dsl.Where, attrToPg, attrPgTypes map[string]string, argStart int) (string, []any, error) {
	colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
	if err != nil {
		return "", nil, err
	}
	op := map[string]string{
		"EQ": " = ", "NEQ": " <> ", "GT": " > ", "GTE": " >= ", "LT": " < ", "LTE": " <= ",
	}[w.Type]
	placeholder := fmt.Sprintf("$%d", argStart)
	if cast := compareArgCast(pgType); cast != "" {
		placeholder += "::" + cast
	}
	return colRef + op + placeholder, []any{w.Val}, nil
}

func compareArgCast(pgType string) string {
	switch strings.ToLower(strings.TrimSpace(pgType)) {
	case "numeric", "bigint", "integer", "int4", "int8", "double precision", "float8":
		return "numeric"
	case "boolean", "bool":
		return "boolean"
	case "timestamptz", "timestamp with time zone", "timestamp", "date":
		return "timestamptz"
	default:
		return ""
	}
}

func buildWhereIn(w dsl.Where, attrToPg, attrPgTypes map[string]string, argStart int) (string, []any, error) {
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
}

func buildWhereBetween(w dsl.Where, attrToPg, attrPgTypes map[string]string, argStart int) (string, []any, error) {
	colRef, _, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
	if err != nil {
		return "", nil, err
	}
	vals, ok := w.Val.([]any)
	if !ok || len(vals) != 2 {
		return "", nil, fmt.Errorf("BETWEEN filter val must be [start, end]")
	}
	return colRef + fmt.Sprintf(" BETWEEN $%d AND $%d", argStart, argStart+1), []any{vals[0], vals[1]}, nil
}

func buildWhereFTS(w dsl.Where, attrToPg, attrPgTypes map[string]string, argStart int) (string, []any, error) {
	colRef, _, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
	if err != nil {
		return "", nil, err
	}
	q := toTsQuery(fmt.Sprint(w.Val))
	if q == "" {
		return "FALSE", nil, nil
	}
	return fmt.Sprintf("to_tsvector('simple', %s) @@ to_tsquery('simple', $%d)", colRef, argStart), []any{q}, nil
}

func toTsQuery(q string) string {
	parts := strings.Fields(q)
	if len(parts) == 0 {
		return ""
	}
	for i, p := range parts {
		p = strings.ReplaceAll(p, "'", "")
		p = strings.ReplaceAll(p, ":", "")
		parts[i] = p
	}
	return strings.Join(parts, " & ")
}

func buildWhereEmpty(w dsl.Where, attrToPg, attrPgTypes map[string]string) (string, []any, error) {
	colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
	if err != nil {
		return "", nil, err
	}
	if isArrayPgType(pgType) {
		cast := arrayCastType(pgType)
		return fmt.Sprintf("(%s IS NULL OR %s = '{}'::%s)", colRef, colRef, cast), nil, nil
	}
	return colRef + " IS NULL", nil, nil
}

func buildWhereNotEmpty(w dsl.Where, attrToPg, attrPgTypes map[string]string) (string, []any, error) {
	colRef, pgType, err := resolveColRefWithType(w.Attr, attrToPg, attrPgTypes)
	if err != nil {
		return "", nil, err
	}
	if isArrayPgType(pgType) {
		cast := arrayCastType(pgType)
		return fmt.Sprintf("(%s IS NOT NULL AND %s <> '{}'::%s)", colRef, colRef, cast), nil, nil
	}
	return colRef + " IS NOT NULL", nil, nil
}

func resolveColRefWithType(attr string, attrToPg, attrPgTypes map[string]string) (colRef string, pgType string, err error) {
	pg, ok := attrToPg[attr]
	if !ok {
		return "", "", fmt.Errorf("unknown filter attribute %q", attr)
	}
	colRef = quoteColRef(pg)
	if attrPgTypes != nil {
		pgType = attrPgTypes[attr]
	}
	return colRef, pgType, nil
}

// quoteColRef sanitizes a bare identifier. JSONB/SQL expressions are left as-is.
func quoteColRef(pg string) string {
	if isRawSQLExpr(pg) {
		return pg
	}
	if strings.Contains(pg, ".") {
		return pg
	}
	return pgx.Identifier{pg}.Sanitize()
}

func isRawSQLExpr(pg string) bool {
	s := strings.TrimSpace(pg)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "(") || strings.HasPrefix(s, "COALESCE(") || strings.HasPrefix(s, "CASE") {
		return true
	}
	if strings.Contains(s, "->") || strings.Contains(s, "::") {
		return true
	}
	if strings.ContainsAny(s, " \t\n") {
		return true
	}
	return false
}

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
