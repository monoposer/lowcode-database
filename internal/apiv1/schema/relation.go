package schema

import "time"

type Relation struct {
	Id             string         `json:"id,omitempty"`
	Name           string         `json:"name,omitempty"`
	Kind           string         `json:"kind,omitempty"`
	SourceTableId  string         `json:"sourceTableId,omitempty"`
	SourceColumnId string         `json:"sourceColumnId,omitempty"`
	TargetTableId  string         `json:"targetTableId,omitempty"`
	TargetColumnId string         `json:"targetColumnId,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
	CreatedAt      time.Time      `json:"createdAt,omitempty"`
	UpdatedAt      time.Time      `json:"updatedAt,omitempty"`
}

type CreateRelationRequest struct {
	Name           string         `json:"name,omitempty"`
	Kind           string         `json:"kind,omitempty"`
	SourceTableId  string         `json:"sourceTableId,omitempty"`
	SourceColumnId string         `json:"sourceColumnId,omitempty"`
	TargetTableId  string         `json:"targetTableId,omitempty"`
	TargetColumnId string         `json:"targetColumnId,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
}

type CreateRelationResponse struct {
	Relation *Relation `json:"relation,omitempty"`
}

type ListRelationsRequest struct {
	TableId string `json:"tableId,omitempty"`
}

type ListRelationsResponse struct {
	Relations []*Relation `json:"relations,omitempty"`
}

type DeleteRelationRequest struct {
	SourceTableId string `json:"sourceTableId,omitempty"`
	Name          string `json:"name,omitempty"`
}

type DeleteRelationResponse struct{}
