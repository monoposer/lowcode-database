package apiv1

import "time"

// -------- Core entities --------

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
	IdType     string    `json:"idType,omitempty"` // logical type id of physical id column (from PG catalog)
	CreatedAt  time.Time `json:"createdAt,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt,omitempty"`
}

type Column struct {
	Id           string         `json:"id,omitempty"`
	TableId      string         `json:"tableId,omitempty"`
	Name         string         `json:"name,omitempty"`
	Label        string         `json:"label,omitempty"`
	TypeId       string         `json:"typeId,omitempty"`
	ResultTypeId string         `json:"resultTypeId,omitempty"` // scalar/array value type for filters and cells
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

type Row struct {
	Id    string            `json:"id,omitempty"`
	Cells map[string]*Value `json:"cells,omitempty"`
}

// -------- Webhook --------

type Webhook struct {
	Id          string         `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	TargetUrl   string         `json:"targetUrl,omitempty"`
	TableFilter string         `json:"tableFilter,omitempty"`
	Events      []string       `json:"events,omitempty"`
	Headers     map[string]any `json:"headers,omitempty"`
	Enabled     bool           `json:"enabled,omitempty"`
	HasSecret   bool           `json:"hasSecret,omitempty"`
	CreatedAt   time.Time      `json:"createdAt,omitempty"`
	UpdatedAt   time.Time      `json:"updatedAt,omitempty"`
}

type CreateWebhookRequest struct {
	Name        string         `json:"name,omitempty"`
	TargetUrl   string         `json:"targetUrl,omitempty"`
	TableFilter string         `json:"tableFilter,omitempty"`
	Events      []string       `json:"events,omitempty"`
	Headers     map[string]any `json:"headers,omitempty"`
	Enabled     bool           `json:"enabled,omitempty"`
	Secret      string         `json:"secret,omitempty"`
}

type CreateWebhookResponse struct {
	Webhook *Webhook `json:"webhook,omitempty"`
}

type ListWebhooksRequest struct{}

type ListWebhooksResponse struct {
	Webhooks []*Webhook `json:"webhooks,omitempty"`
}

type DeleteWebhookRequest struct {
	Id string `json:"id,omitempty"`
}

type DeleteWebhookResponse struct{}

type UpdateWebhookRequest struct {
	Id          string         `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	TargetUrl   string         `json:"targetUrl,omitempty"`
	TableFilter string         `json:"tableFilter,omitempty"`
	Events      []string       `json:"events,omitempty"`
	Headers     map[string]any `json:"headers,omitempty"`
	Enabled     bool           `json:"enabled,omitempty"`
	Secret      string         `json:"secret,omitempty"`
}

type UpdateWebhookResponse struct {
	Webhook *Webhook `json:"webhook,omitempty"`
}

// -------- Tenant --------

type CreateTenantRequest struct {
	Id             string `json:"id,omitempty"`
	DisplayName    string `json:"displayName,omitempty"`
	DataDsn        string `json:"dataDsn,omitempty"`
	CreateDatabase bool   `json:"createDatabase,omitempty"`
}

type CreateTenantResponse struct {
	Id string `json:"id,omitempty"`
}

// -------- Type (built-in, read-only via GET /v1/types) --------

type ListTypesRequest struct{}

type ListTypesResponse struct {
	Types []*Type `json:"types,omitempty"`
}

// -------- Table --------

type CreateTableRequest struct {
	Name       string `json:"name,omitempty"`
	Label      string `json:"label,omitempty"`
	SchemaName string `json:"schemaName,omitempty"`
	IdType     string `json:"idType,omitempty"` // built-in column type id for row id (default uuid)
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

type GetDatabaseConnectionRequest struct{}

type GetDatabaseConnectionResponse struct {
	Host               string `json:"host,omitempty"`
	Port               int32  `json:"port,omitempty"`
	Database           string `json:"database,omitempty"`
	User               string `json:"user,omitempty"`
	UrlWithoutPassword string `json:"urlWithoutPassword,omitempty"`
	PsqlCommand        string `json:"psqlCommand,omitempty"`
	PasswordSourceHint string `json:"passwordSourceHint,omitempty"`
}

type GetTableSchemaRequest struct {
	TableId string `json:"tableId,omitempty"`
}

type GetTableSchemaResponse struct {
	Table   *Table    `json:"table,omitempty"`
	Columns []*Column `json:"columns,omitempty"`
	Indexes []*Index  `json:"indexes,omitempty"`
}

// -------- Column --------

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

// -------- Row --------

type CreateRowRequest struct {
	TableId string            `json:"tableId,omitempty"`
	Cells   map[string]*Value `json:"cells,omitempty"`
}

type CreateRowResponse struct {
	Row *Row `json:"row,omitempty"`
}

type UpdateRowRequest struct {
	TableId string            `json:"tableId,omitempty"`
	RowId   string            `json:"rowId,omitempty"`
	Cells   map[string]*Value `json:"cells,omitempty"`
}

type UpdateRowResponse struct {
	Row *Row `json:"row,omitempty"`
}

type DeleteRowRequest struct {
	TableId string `json:"tableId,omitempty"`
	RowId   string `json:"rowId,omitempty"`
}

type DeleteRowResponse struct{}

type ListRowsRequest struct {
	TableId         string   `json:"tableId,omitempty"`
	PageSize        int32    `json:"pageSize,omitempty"`
	PageToken       string   `json:"pageToken,omitempty"`
	ExpandColumnIds []string `json:"expandColumnIds,omitempty"`
	// ExpandPaths is a list of dot-separated column id paths, e.g. ["relColId","fieldColId"] segments.
	ExpandPaths []string `json:"expandPaths,omitempty"`
}

type ListRowsResponse struct {
	Rows          []*Row `json:"rows,omitempty"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

type BulkUpsertRowItem struct {
	RowId string            `json:"rowId,omitempty"`
	Cells map[string]*Value `json:"cells,omitempty"`
}

type BulkUpsertRowsRequest struct {
	TableId string               `json:"tableId,omitempty"`
	Items   []*BulkUpsertRowItem `json:"items,omitempty"`
}

type BulkUpsertRowsResponse struct {
	Rows []*Row `json:"rows,omitempty"`
}

type BulkDeleteRowsRequest struct {
	TableId string   `json:"tableId,omitempty"`
	RowIds  []string `json:"rowIds,omitempty"`
}

type BulkDeleteRowsResponse struct{}

type ImportRowsFormat int32

const (
	ImportRowsFormatUnspecified ImportRowsFormat = 0
	ImportRowsFormatJSONRows    ImportRowsFormat = 1
)

type ImportRowsRequest struct {
	TableId   string            `json:"tableId,omitempty"`
	Format    ImportRowsFormat  `json:"format,omitempty"`
	Rows      []map[string]any  `json:"rows,omitempty"`
	ColumnMap map[string]string `json:"columnMap,omitempty"`
}

type ImportRowsResponse struct {
	Rows          []*Row `json:"rows,omitempty"`
	InsertedCount int32  `json:"insertedCount,omitempty"`
}

// -------- Index --------

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
	Id string `json:"id,omitempty"` // PG index name
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
