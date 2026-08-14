package catalog

import (
	"time"

	"github.com/monoposer/lowcode-database/internal/columntype"
)

type Index struct {
	Id        string    `json:"id,omitempty"`
	TableName   string    `json:"tableName,omitempty"`
	Name      string    `json:"name,omitempty"`
	PgIndex   string    `json:"pgIndex,omitempty"`
	ColumnIds []string  `json:"columnIds,omitempty"`
	IsUnique  bool      `json:"isUnique,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type IndexBackfillStatus struct {
	Pending     int `json:"pending"`
	Building    int `json:"building"`
	Ready       int `json:"ready"`
	Error       int `json:"error"`
	DropPending int `json:"dropPending"`
}

// ColumnTypeDef is a tenant-defined column type (meta lc_column_types).
type ColumnTypeDef struct {
	Id         string                     `json:"id,omitempty"`
	Name       string                     `json:"name,omitempty"`
	Label      string                     `json:"label,omitempty"`
	BaseId     string                     `json:"baseId,omitempty"`
	SchemaName string                     `json:"schemaName,omitempty"` // deprecated unused
	Spec       *columntype.ColumnTypeSpec `json:"spec,omitempty"`
	RefKind    string                     `json:"refKind,omitempty"` // columnType
	CreatedAt  time.Time                  `json:"createdAt,omitempty"`
	UpdatedAt  time.Time                  `json:"updatedAt,omitempty"`
}

type Type struct {
	Id         string         `json:"id,omitempty"`
	Name       string         `json:"name,omitempty"`
	Label      string         `json:"label,omitempty"`
	PgType     string         `json:"pgType,omitempty"`
	SchemaName string         `json:"schemaName,omitempty"`
	RefKind    string         `json:"refKind,omitempty"` // pgType | columnType | choice | virtual
	Config     map[string]any `json:"config,omitempty"`
	CreatedAt  time.Time      `json:"createdAt,omitempty"`
	UpdatedAt  time.Time      `json:"updatedAt,omitempty"`
}
