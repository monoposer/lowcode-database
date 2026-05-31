package apiv1

import "time"

// -------- Choice (enum) --------

type ChoiceItem struct {
	Name     string `json:"name,omitempty"`
	Label    string `json:"label,omitempty"`
	Value    string `json:"value,omitempty"`
	Color    string `json:"color,omitempty"`
	Order    int32  `json:"order,omitempty"`
	ParentId string `json:"parentId,omitempty"`
}

type ChoiceSource struct {
	Type           string         `json:"type,omitempty"` // STATIC | ENTITY
	TableId        string         `json:"tableId,omitempty"`
	ValueField     string         `json:"valueField,omitempty"`
	LabelField     string         `json:"labelField,omitempty"`
	DataSourceId   string         `json:"dataSourceId,omitempty"`
	Filter         map[string]any `json:"filter,omitempty"`
}

type Choice struct {
	Id         string        `json:"id,omitempty"`
	Name       string        `json:"name,omitempty"`
	Label      string        `json:"label,omitempty"`
	SchemaName string        `json:"schemaName,omitempty"`
	PgTypeName string        `json:"pgTypeName,omitempty"`
	Source     *ChoiceSource `json:"source,omitempty"` // deprecated: enums are always PG ENUM
	Values     []*ChoiceItem `json:"values,omitempty"` // loaded from pg_enum
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
	ReplaceValues bool          `json:"replaceValues,omitempty"` // true: values is the full desired set (add/remove/recreate)
}

type UpdateChoiceResponse struct {
	Choice *Choice `json:"choice,omitempty"`
}

type DeleteChoiceRequest struct {
	Id string `json:"id,omitempty"`
}

type DeleteChoiceResponse struct{}

// -------- Relation --------

type Relation struct {
	Id             string         `json:"id,omitempty"`
	Name           string         `json:"name,omitempty"`
	Kind           string         `json:"kind,omitempty"` // ONE_TO_ONE | ONE_TO_MANY | MANY_TO_ONE
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

// -------- DataSource (list / view definition) --------

type SortOrder struct {
	Attribute string `json:"attribute,omitempty"`
	SortOrder string `json:"sortOrder,omitempty"` // ASC | DESC
}

type DataSource struct {
	Id        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Label     string         `json:"label,omitempty"`
	TableId   string         `json:"tableId,omitempty"`
	Filter    map[string]any `json:"filter,omitempty"`
	Sort      []*SortOrder   `json:"sort,omitempty"`
	ColumnIds []string       `json:"columnIds,omitempty"` // logical column names within tableId (UUID accepted on write)
	Config    map[string]any `json:"config,omitempty"`
	CreatedAt time.Time      `json:"createdAt,omitempty"`
	UpdatedAt time.Time      `json:"updatedAt,omitempty"`
}

type CreateDataSourceRequest struct {
	Name      string         `json:"name,omitempty"`
	Label     string         `json:"label,omitempty"`
	TableId   string         `json:"tableId,omitempty"`
	Filter    map[string]any `json:"filter,omitempty"`
	Sort      []*SortOrder   `json:"sort,omitempty"`
	ColumnIds []string       `json:"columnIds,omitempty"` // logical column names within tableId (UUID accepted on write)
	Config    map[string]any `json:"config,omitempty"`
}

type CreateDataSourceResponse struct {
	DataSource *DataSource `json:"dataSource,omitempty"`
}

type ListDataSourcesRequest struct {
	TableId string `json:"tableId,omitempty"`
}

type ListDataSourcesResponse struct {
	DataSources []*DataSource `json:"dataSources,omitempty"`
}

type GetDataSourceRequest struct {
	TableId string `json:"tableId,omitempty"`
	Name    string `json:"name,omitempty"` // logical id (= name within table_id)
}

type GetDataSourceResponse struct {
	DataSource *DataSource `json:"dataSource,omitempty"`
}

type UpdateDataSourceRequest struct {
	TableId   string         `json:"tableId,omitempty"`
	Name      string         `json:"name,omitempty"`
	Label     string         `json:"label,omitempty"`
	Filter    map[string]any `json:"filter,omitempty"`
	Sort      []*SortOrder   `json:"sort,omitempty"`
	ColumnIds []string       `json:"columnIds,omitempty"` // logical column names within tableId (UUID accepted on write)
	Config    map[string]any `json:"config,omitempty"`
}

type UpdateDataSourceResponse struct {
	DataSource *DataSource `json:"dataSource,omitempty"`
}

type DeleteDataSourceRequest struct {
	TableId string `json:"tableId,omitempty"`
	Name    string `json:"name,omitempty"`
}

type DeleteDataSourceResponse struct{}

type QueryDataSourceRequest struct {
	TableId      string         `json:"tableId,omitempty"`
	DataSourceId string         `json:"dataSourceId,omitempty"` // logical name within tableId
	PageSize     int32          `json:"pageSize,omitempty"`
	PageToken    string         `json:"pageToken,omitempty"`
	Filter       map[string]any `json:"filter,omitempty"`
}

type QueryDataSourceResponse struct {
	Rows          []*Row `json:"rows,omitempty"`
	NextPageToken string `json:"nextPageToken,omitempty"`
	Count         int32  `json:"count,omitempty"`
}

// -------- ER Diagram --------

type ERNode struct {
	TableId   string    `json:"tableId,omitempty"`
	TableName string    `json:"tableName,omitempty"`
	Label     string    `json:"label,omitempty"`
	Columns   []*Column `json:"columns,omitempty"`
}

type EREdge struct {
	Id             string `json:"id,omitempty"`
	Kind           string `json:"kind,omitempty"`
	SourceTableId  string `json:"sourceTableId,omitempty"`
	SourceColumnId string `json:"sourceColumnId,omitempty"`
	TargetTableId  string `json:"targetTableId,omitempty"`
	TargetColumnId string `json:"targetColumnId,omitempty"`
	Label          string `json:"label,omitempty"`
}

type ERDiagram struct {
	Nodes []*ERNode `json:"nodes,omitempty"`
	Edges []*EREdge `json:"edges,omitempty"`
}

type GetERDiagramRequest struct{}

type GetERDiagramResponse struct {
	Diagram *ERDiagram `json:"diagram,omitempty"`
}

// -------- Enhanced query --------

type QueryRowsRequest struct {
	TableId         string         `json:"tableId,omitempty"`
	Filter          map[string]any `json:"filter,omitempty"`
	Sort            []*SortOrder   `json:"sort,omitempty"`
	ColumnIds       []string       `json:"columnIds,omitempty"`
	PageSize        int32          `json:"pageSize,omitempty"`
	PageToken       string         `json:"pageToken,omitempty"`
	ExpandColumnIds []string       `json:"expandColumnIds,omitempty"`
	ExpandPaths     []string       `json:"expandPaths,omitempty"`
}

type QueryRowsResponse struct {
	Rows          []*Row `json:"rows,omitempty"`
	NextPageToken string `json:"nextPageToken,omitempty"`
	Count         int32  `json:"count,omitempty"`
}
