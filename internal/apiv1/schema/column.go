package schema

type AddColumnRequest struct {
	TableId    string         `json:"tableId,omitempty"`
	Name       string         `json:"name,omitempty"`
	Label      string         `json:"label,omitempty"`
	TypeId     string         `json:"typeId,omitempty"`
	IsNullable bool           `json:"isNullable,omitempty"`
	Position   int32          `json:"position,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
}

type AddColumnResponse struct {
	Column *Column `json:"column,omitempty"`
}

type UpdateColumnRequest struct {
	Id         string         `json:"id,omitempty"`
	TableId    string         `json:"tableId,omitempty"`
	Name       string         `json:"name,omitempty"`
	Label      string         `json:"label,omitempty"`
	TypeId     string         `json:"typeId,omitempty"`
	IsNullable *bool          `json:"isNullable,omitempty"`
	Position   int32          `json:"position,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
}

type UpdateColumnResponse struct {
	Column *Column `json:"column,omitempty"`
}

type DeleteColumnRequest struct {
	Id      string `json:"id,omitempty"`
	TableId string `json:"tableId,omitempty"`
}

type DeleteColumnResponse struct{}

type ListColumnsRequest struct {
	TableId string `json:"tableId,omitempty"`
}

type ListColumnsResponse struct {
	Columns []*Column `json:"columns,omitempty"`
}

type CreateIndexRequest struct {
	TableId   string   `json:"tableId,omitempty"`
	Name      string   `json:"name,omitempty"`
	ColumnIds []string `json:"columnIds,omitempty"`
	IsUnique  bool     `json:"isUnique,omitempty"`
}

type CreateIndexResponse struct {
	Index *Index `json:"index,omitempty"`
}

type DeleteIndexRequest struct {
	Id string `json:"id,omitempty"`
}

type DeleteIndexResponse struct{}

type GetIndexRequest struct {
	Id string `json:"id,omitempty"`
}

type GetIndexResponse struct {
	Index *Index `json:"index,omitempty"`
}

type ListIndexesRequest struct {
	TableId string `json:"tableId,omitempty"`
}

type ListIndexesResponse struct {
	Indexes []*Index `json:"indexes,omitempty"`
}
