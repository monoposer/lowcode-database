package schema

import (
	"time"

	"github.com/monoposer/lowcode-database/pkg/typespec"
)

// ColumnTypeDef is a tenant-defined column type (PG DOMAIN + meta lc_column_types).
type ColumnTypeDef struct {
	Id         string                   `json:"id,omitempty"`
	Name       string                   `json:"name,omitempty"`
	Label      string                   `json:"label,omitempty"`
	BaseId     string                   `json:"baseId,omitempty"`
	SchemaName string                   `json:"schemaName,omitempty"` // deprecated unused
	Spec       *typespec.ColumnTypeSpec `json:"spec,omitempty"`
	RefKind    string                   `json:"refKind,omitempty"` // columnType
	CreatedAt  time.Time                `json:"createdAt,omitempty"`
	UpdatedAt  time.Time                `json:"updatedAt,omitempty"`
}

type CreateColumnTypeRequest struct {
	Name       string                   `json:"name,omitempty"`
	Label      string                   `json:"label,omitempty"`
	BaseId     string                   `json:"baseId,omitempty"`
	SchemaName string                   `json:"schemaName,omitempty"` // deprecated unused
	Spec       *typespec.ColumnTypeSpec `json:"spec,omitempty"`
}

type CreateColumnTypeResponse struct {
	ColumnType *ColumnTypeDef `json:"columnType,omitempty"`
}

type ListColumnTypesRequest struct{}

type ListColumnTypesResponse struct {
	ColumnTypes []*ColumnTypeDef `json:"columnTypes,omitempty"`
}

type GetColumnTypeRequest struct {
	Id string `json:"id,omitempty"`
}

type GetColumnTypeResponse struct {
	ColumnType *ColumnTypeDef `json:"columnType,omitempty"`
}

type UpdateColumnTypeRequest struct {
	Id    string                   `json:"id,omitempty"`
	Label string                   `json:"label,omitempty"`
	Spec  *typespec.ColumnTypeSpec `json:"spec,omitempty"`
}

type UpdateColumnTypeResponse struct {
	ColumnType *ColumnTypeDef `json:"columnType,omitempty"`
}

type DeleteColumnTypeRequest struct {
	Id string `json:"id,omitempty"`
}

type DeleteColumnTypeResponse struct{}

type ImportTypeCatalogRequest struct {
	Catalog *typespec.TypeCatalog `json:"catalog,omitempty"`
}

type ImportTypeCatalogResponse struct {
	ColumnTypesCreated int `json:"columnTypesCreated,omitempty"`
}
