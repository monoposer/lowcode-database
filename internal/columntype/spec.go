package columntype

const (
	APIVersion = "typespec.lowcode/v1"

	KindColumnType  = "ColumnType"
	KindTypeCatalog = "TypeCatalog"

	RefKindPgType     = "pgType"
	RefKindColumnType = "columnType"
	RefKindVirtual    = "virtual"
)

// Category classifies a built-in pgType entry.
type Category string

const (
	CategoryScalar  Category = "scalar"
	CategoryArray   Category = "array"
	CategoryVirtual Category = "virtual"
)

// Modifier describes an optional parameter on a pgType (e.g. precision/scale on numeric).
type Modifier struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // integer | boolean | string
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
}

// PgTypeEntry is a platform built-in PostgreSQL type (not stored in meta DB).
type PgTypeEntry struct {
	ID          string     `json:"id"`
	Name        string     `json:"name,omitempty"`
	PgType      string     `json:"pgType"`
	Category    Category   `json:"category"`
	Description string     `json:"description,omitempty"`
	Modifiers   []Modifier `json:"modifiers,omitempty"`
	Kind        string     `json:"kind,omitempty"` // virtual: formula, link, lookup, rollup
}

// CheckSpec is one CHECK on a tenant column type. Expr uses VALUE.
type CheckSpec struct {
	Name string `json:"name,omitempty"`
	Expr string `json:"expr"`
}

// ColumnTypeMetadata identifies a tenant-defined column type.
type ColumnTypeMetadata struct {
	Name        string `json:"name"`
	Label       string `json:"label,omitempty"`
	SchemaName  string `json:"schemaName,omitempty"`
	Description string `json:"description,omitempty"`
}

// ColumnTypeSpec is the definition of a tenant column type (underlying pgType + CHECKs).
type ColumnTypeSpec struct {
	// PgType is the underlying built-in scalar id: text|number|datetime|boolean|jsonb.
	// Not virtual, and not raw SQL.
	PgType    string      `json:"pgType"`
	Array     bool        `json:"array,omitempty"` // true → logical PG array (e.g. text[])
	Precision *int        `json:"precision,omitempty"`
	Scale     *int        `json:"scale,omitempty"`
	NotNull   bool        `json:"notNull,omitempty"`
	Default   *string     `json:"default,omitempty"`
	Checks    []CheckSpec `json:"checks,omitempty"`
}

// ColumnType is the import document for one tenant column type.
type ColumnType struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Metadata   ColumnTypeMetadata `json:"metadata"`
	Spec       ColumnTypeSpec     `json:"spec"`
}

// TypeCatalog bundles tenant column types for Admin import.
type TypeCatalog struct {
	APIVersion  string       `json:"apiVersion"`
	Kind        string       `json:"kind"`
	ColumnTypes []ColumnType `json:"columnTypes,omitempty"`
}
