package schema

import "time"

type Type struct {
	Id        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	PgType    string         `json:"pgType,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
	CreatedAt time.Time      `json:"createdAt,omitempty"`
	UpdatedAt time.Time      `json:"updatedAt,omitempty"`
}

type Table struct {
	Id         string    `json:"id,omitempty"`
	Name       string    `json:"name,omitempty"`
	Label      string    `json:"label,omitempty"`
	SchemaName string    `json:"schemaName,omitempty"`
	IdType     string    `json:"idType,omitempty"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt,omitempty"`
}

type Column struct {
	Id           string         `json:"id,omitempty"`
	TableId      string         `json:"tableId,omitempty"`
	Name         string         `json:"name,omitempty"`
	Label        string         `json:"label,omitempty"`
	TypeId       string         `json:"typeId,omitempty"`
	ResultTypeId string         `json:"resultTypeId,omitempty"`
	IsNullable   bool           `json:"isNullable,omitempty"`
	Position     int32          `json:"position,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
	CreatedAt    time.Time      `json:"createdAt,omitempty"`
	UpdatedAt    time.Time      `json:"updatedAt,omitempty"`
}

type Index struct {
	Id        string    `json:"id,omitempty"`
	TableId   string    `json:"tableId,omitempty"`
	Name      string    `json:"name,omitempty"`
	PgIndex   string    `json:"pgIndex,omitempty"`
	ColumnIds []string  `json:"columnIds,omitempty"`
	IsUnique  bool      `json:"isUnique,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}
