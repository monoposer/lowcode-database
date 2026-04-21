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
	ID       string
	PgColumn string
	PgType   string
}

// OrderSpec is a sort directive.
type OrderSpec struct {
	Attribute string
	SortOrder string // ASC | DESC
}

// BuildWhere renders a WHERE clause (without the WHERE keyword) and args starting at argStart.
func BuildWhere(w dsl.Where, attrToPg map[string]string, argStart int) (string, []any, error) {
	if w.Type == "" {
		return "", nil, nil
	}
	switch w.Type {
	case "AND", "OR":
		var parts []string
		var args []any
		idx := argStart
		for _, child := range w.Vals {
			part, childArgs, err := BuildWhere(child, attrToPg, idx)
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
	case "EQ", "NEQ", "GT", "GTE", "LT", "LTE", "LIKE":
		pg, ok := attrToPg[w.Attr]
		if !ok {
			return "", nil, fmt.Errorf("unknown filter attribute %q", w.Attr)
		}
		op := map[string]string{
			"EQ": " = ", "NEQ": " <> ", "GT": " > ", "GTE": " >= ", "LT": " < ", "LTE": " <= ", "LIKE": " LIKE ",
		}[w.Type]
		if !strings.Contains(pg, ".") {
			pg = pgx.Identifier{pg}.Sanitize()
		}
		return pg + op + fmt.Sprintf("$%d", argStart), []any{w.Val}, nil
	case "IN", "NIN":
		pg, ok := attrToPg[w.Attr]
		if !ok {
			return "", nil, fmt.Errorf("unknown filter attribute %q", w.Attr)
		}
		vals, ok := w.Val.([]any)
		if !ok {
			// try []interface{} from json
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
		colRef := pg
		if !strings.Contains(pg, ".") {
			colRef = pgx.Identifier{pg}.Sanitize()
		}
		return colRef + op + "(" + strings.Join(placeholders, ", ") + ")", args, nil
	case "EMPTY":
		pg, ok := attrToPg[w.Attr]
		if !ok {
			return "", nil, fmt.Errorf("unknown filter attribute %q", w.Attr)
		}
		colRef := pg
		if !strings.Contains(pg, ".") {
			colRef = pgx.Identifier{pg}.Sanitize()
		}
		return colRef + " IS NULL", nil, nil
	case "NOT_EMPTY":
		pg, ok := attrToPg[w.Attr]
		if !ok {
			return "", nil, fmt.Errorf("unknown filter attribute %q", w.Attr)
		}
		colRef := pg
		if !strings.Contains(pg, ".") {
			colRef = pgx.Identifier{pg}.Sanitize()
		}
		return colRef + " IS NOT NULL", nil, nil
	default:
		return "", nil, fmt.Errorf("unsupported filter type %q", w.Type)
	}
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
		ref := alias + "." + c.PgColumn
		m[c.ID] = ref
		m[c.PgColumn] = ref
	}
	return m
}
