package schema

import "time"

type ChoiceItem struct {
	Name     string `json:"name,omitempty"`
	Label    string `json:"label,omitempty"`
	Value    string `json:"value,omitempty"`
	Color    string `json:"color,omitempty"`
	Order    int32  `json:"order,omitempty"`
	ParentId string `json:"parentId,omitempty"`
}

type ChoiceSource struct {
	Type         string         `json:"type,omitempty"`
	TableId      string         `json:"tableId,omitempty"`
	ValueField   string         `json:"valueField,omitempty"`
	LabelField   string         `json:"labelField,omitempty"`
	DataSourceId string         `json:"dataSourceId,omitempty"`
	Filter       map[string]any `json:"filter,omitempty"`
}

type Choice struct {
	Id         string        `json:"id,omitempty"`
	Name       string        `json:"name,omitempty"`
	Label      string        `json:"label,omitempty"`
	SchemaName string        `json:"schemaName,omitempty"`
	PgTypeName string        `json:"pgTypeName,omitempty"`
	Source     *ChoiceSource `json:"source,omitempty"`
	Values     []*ChoiceItem `json:"values,omitempty"`
	CreatedAt  time.Time     `json:"createdAt,omitempty"`
	UpdatedAt  time.Time     `json:"updatedAt,omitempty"`
}

type CreateChoiceRequest struct {
	Name       string        `json:"name,omitempty"`
	Label      string        `json:"label,omitempty"`
	SchemaName string        `json:"schemaName,omitempty"`
	Values     []*ChoiceItem `json:"values,omitempty"`
}

type CreateChoiceResponse struct {
	Choice *Choice `json:"choice,omitempty"`
}

type ListChoicesRequest struct{}

type ListChoicesResponse struct {
	Choices []*Choice `json:"choices,omitempty"`
}

type GetChoiceRequest struct {
	Id string `json:"id,omitempty"`
}

type GetChoiceResponse struct {
	Choice *Choice `json:"choice,omitempty"`
}

type UpdateChoiceRequest struct {
	Id            string        `json:"id,omitempty"`
	Label         string        `json:"label,omitempty"`
	Values        []*ChoiceItem `json:"values,omitempty"`
	ReplaceValues bool          `json:"replaceValues,omitempty"`
}

type UpdateChoiceResponse struct {
	Choice *Choice `json:"choice,omitempty"`
}

type DeleteChoiceRequest struct {
	Id string `json:"id,omitempty"`
}

type DeleteChoiceResponse struct{}
