package schema

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
