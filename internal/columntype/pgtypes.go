package columntype

import (
	"fmt"
	"strings"
)

var canonicalPgTypes = []PgTypeEntry{
	{ID: "text", Name: "text", PgType: "text", Category: CategoryScalar, Description: "Unbounded UTF-8 text"},
	{ID: "number", Name: "number", PgType: "numeric", Category: CategoryScalar, Description: "Exact decimal",
		Modifiers: []Modifier{
			{Name: "precision", Type: "integer", Default: 20},
			{Name: "scale", Type: "integer", Default: 6},
		}},
	{ID: "datetime", Name: "datetime", PgType: "timestamptz", Category: CategoryScalar, Description: "Timestamp with time zone"},
	{ID: "boolean", Name: "boolean", PgType: "boolean", Category: CategoryScalar},
	{ID: "jsonb", Name: "jsonb", PgType: "jsonb", Category: CategoryScalar},
	{ID: "formula", Name: "formula", Category: CategoryVirtual, Kind: "formula"},
	{ID: "link", Name: "link", Category: CategoryVirtual, Kind: "link"},
	{ID: "lookup", Name: "lookup", Category: CategoryVirtual, Kind: "lookup"},
	{ID: "rollup", Name: "rollup", Category: CategoryVirtual, Kind: "rollup"},
}

var pgTypeByID map[string]PgTypeEntry

func init() {
	pgTypeByID = make(map[string]PgTypeEntry, len(canonicalPgTypes))
	for _, t := range canonicalPgTypes {
		if t.Name == "" {
			t.Name = t.ID
		}
		pgTypeByID[t.ID] = t
	}
}

// ListPgTypes returns canonical built-in types.
func ListPgTypes() []PgTypeEntry {
	out := make([]PgTypeEntry, len(canonicalPgTypes))
	copy(out, canonicalPgTypes)
	return out
}

// GetPgType resolves a built-in type id.
func GetPgType(id string) (PgTypeEntry, bool) {
	t, ok := pgTypeByID[id]
	return t, ok
}

// CanonicalID returns the built-in type id, or the input if unknown.
func CanonicalID(id string) string {
	if _, ok := pgTypeByID[id]; ok {
		return id
	}
	return id
}

// IsLinkType reports whether typeId is the Link column type.
func IsLinkType(typeID string) bool {
	if typeID == "link" {
		return true
	}
	t, ok := GetPgType(typeID)
	return ok && t.Kind == "link"
}

// EffectiveColumnTypePgType returns the logical PostgreSQL type for a columnType spec
// (e.g. text + array → text[]). PgType must be text|number|datetime|boolean|jsonb.
func EffectiveColumnTypePgType(spec ColumnTypeSpec) (string, error) {
	pg := strings.TrimSpace(spec.PgType)
	if canon, ok := NormalizeColumnTypePgType(pg); ok {
		pg = canon
	} else if pg != "" {
		return "", fmt.Errorf("spec.pgType must be one of text, number, datetime, boolean, jsonb")
	}
	underlying, err := ResolveUnderlyingPgType(pg, spec.Precision, spec.Scale)
	if err != nil {
		return "", err
	}
	if !spec.Array {
		return underlying, nil
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(underlying)), "[]") {
		return underlying, nil
	}
	return underlying + "[]", nil
}
