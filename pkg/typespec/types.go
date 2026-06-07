package typespec

const (
	// APIVersion is the current typespec contract version.
	APIVersion = "typespec.lowcode/v1"

	KindColumnType  = "ColumnType"
	KindTypeCatalog = "TypeCatalog"
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
	ID           string     `json:"id"`
	Name         string     `json:"name,omitempty"`
	PgType       string     `json:"pgType"`
	Category     Category   `json:"category"`
	Description  string     `json:"description,omitempty"`
	Modifiers    []Modifier `json:"modifiers,omitempty"`
	Kind         string     `json:"kind,omitempty"` // virtual: formula, link, lookup, rollup
	Deprecated   bool       `json:"deprecated,omitempty"`
	AliasOf      string     `json:"aliasOf,omitempty"`
	ArrayElement string     `json:"arrayElement,omitempty"`
}

// CheckSpec is one CHECK on a tenant column type (stored as PG DOMAIN). Expr uses VALUE.
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

// ColumnTypeSpec is the portable definition of a tenant column type.
//
// pgType is the underlying PostgreSQL type id (text, numeric, …) or raw PG (text[], numeric(18,2)).
// Column typeId = metadata.name. Implementation uses CREATE DOMAIN in PG.
type ColumnTypeSpec struct {
	PgType    string      `json:"pgType"`
	Precision *int        `json:"precision,omitempty"`
	Scale     *int        `json:"scale,omitempty"`
	NotNull   bool        `json:"notNull,omitempty"`
	Default   *string     `json:"default,omitempty"`
	Checks    []CheckSpec `json:"checks,omitempty"`
}

// ColumnType is the top-level portable document for one tenant column type.
type ColumnType struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Metadata   ColumnTypeMetadata `json:"metadata"`
	Spec       ColumnTypeSpec     `json:"spec"`
}

// TypeCatalog bundles tenant column types for low-code init/import.
type TypeCatalog struct {
	APIVersion  string       `json:"apiVersion"`
	Kind        string       `json:"kind"`
	ColumnTypes []ColumnType `json:"columnTypes,omitempty"`
}

// ColumnTypeRef describes how a column references a type (for docs/codegen).
type ColumnTypeRef struct {
	TypeID     string `json:"typeId"`
	RefKind    string `json:"refKind"` // pgType | columnType | virtual
	PgTypeSQL  string `json:"pgTypeSql,omitempty"`
	SchemaName string `json:"schemaName,omitempty"`
}

const (
	RefKindPgType     = "pgType"
	RefKindColumnType = "columnType"
	RefKindVirtual    = "virtual"
)
