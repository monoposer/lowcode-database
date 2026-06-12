package schema

type CreateTableRequest struct {
	Name       string `json:"name,omitempty"`
	Label      string `json:"label,omitempty"`
	SchemaName string `json:"schemaName,omitempty"`
	IdType     string `json:"idType,omitempty"`
}

type CreateTableResponse struct {
	Table *Table `json:"table,omitempty"`
}

type DeleteTableRequest struct {
	Id string `json:"id,omitempty"`
}

type DeleteTableResponse struct{}

type ListTablesRequest struct{}

type ListTablesResponse struct {
	Tables []*Table `json:"tables,omitempty"`
}

type RenameTableRequest struct {
	Id      string `json:"id,omitempty"`
	NewName string `json:"newName,omitempty"`
}

type RenameTableResponse struct {
	Table *Table `json:"table,omitempty"`
}

type GetTableSchemaRequest struct {
	TableId string `json:"tableId,omitempty"`
}

type GetTableSchemaResponse struct {
	Table   *Table    `json:"table,omitempty"`
	Columns []*Column `json:"columns,omitempty"`
	Indexes []*Index  `json:"indexes,omitempty"`
}
