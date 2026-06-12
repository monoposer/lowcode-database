package query

import (
	"github.com/jackc/pgx/v5"
	"strings"
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
