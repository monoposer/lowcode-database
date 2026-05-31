package columntype

import (
	"fmt"
	"maps"
	"sort"
)

// Type is a built-in column type (not stored in DB).
type Type struct {
	ID     string
	Name   string
	PgType string
	Kind   string // formula, relationship, lookup, rollup, relation_fk; empty for scalars
	Config map[string]any
}

var registry = map[string]Type{}

func init() {
	for _, t := range []Type{
		// scalars
		{ID: "text", Name: "text", PgType: "text"},
		{ID: "number", Name: "number", PgType: "numeric"},
		{ID: "bool", Name: "bool", PgType: "boolean"},
		{ID: "timestamp", Name: "timestamp", PgType: "timestamptz"},
		{ID: "json", Name: "json", PgType: "jsonb"},
		{ID: "uuid", Name: "uuid", PgType: "uuid"},
		{ID: "integer", Name: "integer", PgType: "bigint"},
		{ID: "date", Name: "date", PgType: "date"},
		{ID: "bytea", Name: "bytea", PgType: "bytea"},
		{ID: "int8", Name: "int8", PgType: "bigint"},
		{ID: "double", Name: "double", PgType: "double precision"},
		{ID: "precision", Name: "precision", PgType: "numeric", Config: map[string]any{"precision": 20, "scale": 6}},
		{ID: "timestamptz", Name: "timestamptz", PgType: "timestamptz"},
		{ID: "jsonb", Name: "jsonb", PgType: "jsonb"},
		// arrays
		{ID: "int8_array", Name: "int8_array", PgType: "bigint[]", Config: map[string]any{"array": true}},
		{ID: "double_array", Name: "double_array", PgType: "double precision[]", Config: map[string]any{"array": true}},
		{ID: "text_array", Name: "text_array", PgType: "text[]", Config: map[string]any{"array": true}},
		{ID: "bool_array", Name: "bool_array", PgType: "boolean[]", Config: map[string]any{"array": true}},
		{ID: "jsonb_array", Name: "jsonb_array", PgType: "jsonb[]", Config: map[string]any{"array": true}},
		{ID: "timestamptz_array", Name: "timestamptz_array", PgType: "timestamptz[]", Config: map[string]any{"array": true}},
		{ID: "uuid_array", Name: "uuid_array", PgType: "uuid[]", Config: map[string]any{"array": true}},
		// postgis
		{ID: "geometry", Name: "geometry", PgType: "geometry", Config: map[string]any{"postgis": true}},
		{ID: "geography", Name: "geography", PgType: "geography", Config: map[string]any{"postgis": true}},
		{ID: "point", Name: "point", PgType: "geometry(Point,4326)", Config: map[string]any{"postgis": true}},
		// virtual — no physical PG column; PgType empty
		{ID: "formula", Name: "formula", Kind: "formula"},
		{ID: "relationship", Name: "relationship", Kind: "relationship"},
		{ID: "lookup", Name: "lookup", Kind: "lookup"},
		{ID: "rollup", Name: "rollup", Kind: "rollup"},
		{ID: "relation_fk", Name: "relation_fk", Kind: "relation_fk"},
	} {
		register(t)
	}
}

func register(t Type) {
	if t.Name == "" {
		t.Name = t.ID
	}
	if t.Config == nil {
		t.Config = map[string]any{}
	}
	if t.Kind != "" {
		t.Config["kind"] = t.Kind
	}
	registry[t.ID] = t
}

// Get returns a built-in type by id (same as name).
func Get(id string) (Type, bool) {
	t, ok := registry[id]
	return t, ok
}

// Resolve validates type id and returns type metadata.
func Resolve(id string) (Type, error) {
	t, ok := Get(id)
	if !ok {
		return Type{}, fmt.Errorf("unknown column type %q", id)
	}
	return t, nil
}

// List returns all built-in types sorted by name.
func List() []Type {
	out := make([]Type, 0, len(registry))
	for _, t := range registry {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Kind returns the logical kind for a type id (may be empty for scalars).
func Kind(id string) string {
	if t, ok := Get(id); ok {
		return t.Kind
	}
	return ""
}

// IsBuiltIn reports whether id is a registered built-in column type.
func IsBuiltIn(id string) bool {
	_, ok := Get(id)
	return ok
}

// PgType returns the default PostgreSQL type name for a type id.
// Virtual columns return "" (no fixed PG type). Choice columns use type_id = choice name — resolve via service.
func PgType(id string) string {
	if IsVirtual(id) {
		return ""
	}
	if t, ok := Get(id); ok {
		return t.PgType
	}
	return ""
}

// Config returns a copy of the type config map.
func Config(id string) map[string]any {
	t, ok := Get(id)
	if !ok || t.Config == nil {
		return map[string]any{}
	}
	return maps.Clone(t.Config)
}

// IsVirtual reports whether the type id is a virtual column (no physical PG column semantics).
func IsVirtual(id string) bool {
	return IsVirtualKind(Kind(id))
}

// IsVirtualKind reports whether a column kind string denotes a virtual column.
func IsVirtualKind(kind string) bool {
	switch kind {
	case "formula", "relationship", "lookup", "rollup":
		return true
	default:
		return false
	}
}
