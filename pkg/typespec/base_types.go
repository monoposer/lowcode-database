// Canonical built-in types — keep small. Tenant validation → ColumnType via Admin API.
package typespec

import "strings"

var arrayModifier = Modifier{Name: "array", Type: "boolean", Default: false, Description: "Store as PostgreSQL array"}

var canonicalPgTypes = []PgTypeEntry{
	{ID: "text", Name: "text", PgType: "text", Category: CategoryScalar, Description: "Unbounded UTF-8 text",
		Modifiers: []Modifier{arrayModifier}},
	{ID: "number", Name: "number", PgType: "numeric", Category: CategoryScalar, Description: "Exact decimal",
		Modifiers: []Modifier{
			{Name: "precision", Type: "integer", Default: 20},
			{Name: "scale", Type: "integer", Default: 6},
			arrayModifier,
		}},
	{ID: "datetime", Name: "datetime", PgType: "timestamptz", Category: CategoryScalar, Description: "Timestamp with time zone",
		Modifiers: []Modifier{arrayModifier}},
	{ID: "boolean", Name: "boolean", PgType: "boolean", Category: CategoryScalar,
		Modifiers: []Modifier{arrayModifier}},
	{ID: "jsonb", Name: "jsonb", PgType: "jsonb", Category: CategoryScalar,
		Modifiers: []Modifier{arrayModifier}},
	{ID: "point", Name: "point", PgType: "geometry(Point,4326)", Category: CategoryScalar, Description: "PostGIS point",
		Modifiers: []Modifier{arrayModifier}},
	{ID: "formula", Name: "formula", Category: CategoryVirtual, Kind: "formula"},
	{ID: "link", Name: "link", Category: CategoryVirtual, Kind: "link"},
	{ID: "lookup", Name: "lookup", Category: CategoryVirtual, Kind: "lookup"},
	{ID: "rollup", Name: "rollup", Category: CategoryVirtual, Kind: "rollup"},
}

// legacyPgTypeAliases maps deprecated type ids to a canonical id.
var legacyPgTypeAliases = map[string]string{
	"numeric": "number", "bigint": "number", "float8": "number", "integer": "number",
	"int8": "number", "double": "number", "precision": "number",
	"timestamptz": "datetime", "timestamp": "datetime", "date": "datetime",
	"bool": "boolean", "json": "jsonb",
	"uuid": "text", "bytea": "text",
	"geometry": "point", "geography": "point",
	"text_array": "text", "bool_array": "boolean", "boolean_array": "boolean",
	"int8_array": "number", "double_array": "number", "number_array": "number",
	"jsonb_array": "jsonb", "timestamptz_array": "datetime", "datetime_array": "datetime",
	"uuid_array": "text", "point_array": "point",
	"relationship": "link", "relation_fk": "link",
}

// legacyPgOverride keeps the original PG type for stored legacy ids.
var legacyPgOverride = map[string]string{
	"numeric": "numeric", "bigint": "bigint", "int8": "bigint", "integer": "bigint",
	"float8": "double precision", "double": "double precision", "precision": "numeric",
	"timestamptz": "timestamptz", "timestamp": "timestamptz", "date": "date",
	"uuid": "uuid", "bytea": "bytea", "geometry": "geometry", "geography": "geography",
}

var legacyArrayPg = map[string]string{
	"text_array": "text[]", "bool_array": "boolean[]", "boolean_array": "boolean[]",
	"int8_array": "bigint[]", "double_array": "double precision[]", "number_array": "numeric[]",
	"jsonb_array": "jsonb[]", "timestamptz_array": "timestamptz[]", "datetime_array": "timestamptz[]",
	"uuid_array": "uuid[]", "point_array": "geometry(Point,4326)[]",
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

// GetPgType resolves a type id (including legacy aliases).
func GetPgType(id string) (PgTypeEntry, bool) {
	if t, ok := pgTypeByID[id]; ok {
		return t, true
	}
	if canon, ok := legacyPgTypeAliases[id]; ok {
		t, ok := pgTypeByID[canon]
		if !ok {
			return PgTypeEntry{}, false
		}
		alias := t
		alias.ID = id
		alias.Deprecated = true
		alias.AliasOf = canon
		if pg, ok := legacyPgOverride[id]; ok {
			alias.PgType = pg
		}
		if pg, ok := legacyArrayPg[id]; ok {
			alias.PgType = pg
			alias.Category = CategoryArray
			alias.ArrayElement = canon
		}
		return alias, true
	}
	return PgTypeEntry{}, false
}

// CanonicalID maps a type id (including aliases and *_array) to the catalog id.
func CanonicalID(id string) string {
	t, ok := GetPgType(id)
	if !ok {
		return id
	}
	if t.AliasOf != "" {
		return t.AliasOf
	}
	return t.ID
}

// AllowsArray reports whether a canonical type may use the array modifier.
func AllowsArray(id string) bool {
	t, ok := GetPgType(CanonicalID(id))
	if !ok || t.Category != CategoryScalar {
		return false
	}
	for _, m := range t.Modifiers {
		if m.Name == "array" {
			return true
		}
	}
	return false
}

// IsArrayType reports a legacy *_array id or CategoryArray alias.
func IsArrayType(id string) bool {
	if strings.HasSuffix(id, "_array") {
		return true
	}
	t, ok := GetPgType(id)
	return ok && t.Category == CategoryArray
}

// ListBaseTypes is an alias for ListPgTypes (used by columntype bridge).
func ListBaseTypes() []PgTypeEntry { return ListPgTypes() }

// GetBaseType is an alias for GetPgType (used by columntype bridge).
func GetBaseType(id string) (PgTypeEntry, bool) { return GetPgType(id) }

// IsLegacyAlias reports whether id is a deprecated type alias.
func IsLegacyAlias(id string) bool {
	_, ok := legacyPgTypeAliases[id]
	return ok
}

// IsVirtualKind reports virtual column kinds (includes legacy relationship / relation_fk ids).
func IsVirtualKind(kind string) bool {
	switch kind {
	case "formula", "link", "lookup", "rollup", "relationship", "relation_fk":
		return true
	default:
		return false
	}
}

// IsLinkType reports whether typeId is the Link column type (including legacy ids).
func IsLinkType(typeID string) bool {
	if typeID == "link" || typeID == "relationship" || typeID == "relation_fk" {
		return true
	}
	t, ok := GetPgType(typeID)
	return ok && (t.ID == "link" || t.AliasOf == "link" || t.Kind == "link")
}

// BaseCatalog returns a TypeCatalog shell for export/docs.
func BaseCatalog() TypeCatalog {
	return TypeCatalog{APIVersion: APIVersion, Kind: KindTypeCatalog}
}
