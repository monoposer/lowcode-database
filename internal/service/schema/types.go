package schema

import "time"

type Table struct {
	Id         string    `json:"id,omitempty"`
	Name       string    `json:"name,omitempty"`
	Label      string    `json:"label,omitempty"`
	BaseId     string    `json:"baseId,omitempty"`
	SchemaName string    `json:"schemaName,omitempty"` // deprecated unused
	IdType     string    `json:"idType,omitempty"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt,omitempty"`
}

type Column struct {
	Id           string         `json:"id,omitempty"`
	TableName      string         `json:"tableName,omitempty"`
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

type Relation struct {
	Id             string         `json:"id,omitempty"`
	Name           string         `json:"name,omitempty"`
	Kind           string         `json:"kind,omitempty"`
	SourceTableName  string         `json:"sourceTableName,omitempty"`
	SourceColumnId string         `json:"sourceColumnId,omitempty"`
	TargetTableName  string         `json:"targetTableName,omitempty"`
	TargetColumnId string         `json:"targetColumnId,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
	CreatedAt      time.Time      `json:"createdAt,omitempty"`
	UpdatedAt      time.Time      `json:"updatedAt,omitempty"`
}

type ERNode struct {
	TableName string    `json:"tableName,omitempty"`
	Label     string    `json:"label,omitempty"`
	Columns   []*Column `json:"columns,omitempty"`
}

type EREdge struct {
	Id             string `json:"id,omitempty"`
	Kind           string `json:"kind,omitempty"`
	SourceTableName  string `json:"sourceTableName,omitempty"`
	SourceColumnId string `json:"sourceColumnId,omitempty"`
	TargetTableName  string `json:"targetTableName,omitempty"`
	TargetColumnId string `json:"targetColumnId,omitempty"`
	Label          string `json:"label,omitempty"`
}

type ERDiagram struct {
	Nodes []*ERNode `json:"nodes,omitempty"`
	Edges []*EREdge `json:"edges,omitempty"`
}
