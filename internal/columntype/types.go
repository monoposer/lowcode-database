package columntype

import (
	"fmt"
	"maps"
	"sort"
)

// Type is a built-in column type. Tenant columnTypes live in lc_column_types.
type Type struct {
	ID     string
	Name   string
	PgType string
	Kind   string // formula, link, lookup, rollup; empty for scalars
	Config map[string]any
}

var registry = map[string]Type{}

func init() {
	for _, pt := range ListPgTypes() {
		register(fromPgTypeEntry(pt))
	}
}

func fromPgTypeEntry(pt PgTypeEntry) Type {
	t := Type{
		ID:     pt.ID,
		Name:   pt.Name,
		PgType: pt.PgType,
		Kind:   pt.Kind,
		Config: map[string]any{},
	}
	if pt.Kind != "" {
		t.Config["kind"] = pt.Kind
	}
	for _, m := range pt.Modifiers {
		if m.Default == nil {
			continue
		}
		switch m.Name {
		case "precision", "scale", "financialMode", "roundingMode":
			t.Config[m.Name] = m.Default
		}
	}
	return t
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

// Get returns a built-in pgType by id.
func Get(id string) (Type, bool) {
	t, ok := registry[id]
	return t, ok
}

// Resolve validates type id and returns built-in type metadata.
func Resolve(id string) (Type, error) {
	t, ok := Get(id)
	if !ok {
		return Type{}, fmt.Errorf("unknown column type %q", id)
	}
	return t, nil
}

// List returns canonical built-in pgTypes (no tenant columnTypes).
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
	_, ok := registry[id]
	return ok
}

// PgType returns the default PostgreSQL type name for a built-in type id.
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

// IsVirtual reports whether the type id is a virtual column.
func IsVirtual(id string) bool {
	return IsVirtualKind(Kind(id))
}

// IsVirtualKind reports whether a column kind string denotes a virtual column.
func IsVirtualKind(kind string) bool {
	switch kind {
	case "formula", "link", "lookup", "rollup":
		return true
	default:
		return false
	}
}
