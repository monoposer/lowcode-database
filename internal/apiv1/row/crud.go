package row

import (
	"github.com/solat/lowcode-database/internal/apiv1"
)

type CreateRowRequest struct {
	TableId string                  `json:"tableId,omitempty"`
	Cells   map[string]*apiv1.Value `json:"cells,omitempty"`
}

type CreateRowResponse struct {
	Row *Row `json:"row,omitempty"`
}

type UpdateRowRequest struct {
	TableId string                  `json:"tableId,omitempty"`
	RowId   string                  `json:"rowId,omitempty"`
	Cells   map[string]*apiv1.Value `json:"cells,omitempty"`
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
	ExpandPaths     []string `json:"expandPaths,omitempty"`
}

type ListRowsResponse struct {
	Rows          []*Row `json:"rows,omitempty"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

type BulkUpsertRowItem struct {
	RowId string                  `json:"rowId,omitempty"`
	Cells map[string]*apiv1.Value `json:"cells,omitempty"`
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

type QueryRowsRequest struct {
	TableId         string             `json:"tableId,omitempty"`
	Filter          map[string]any     `json:"filter,omitempty"`
	Sort            []*apiv1.SortOrder `json:"sort,omitempty"`
	ColumnIds       []string           `json:"columnIds,omitempty"`
	PageSize        int32              `json:"pageSize,omitempty"`
	PageToken       string             `json:"pageToken,omitempty"`
	ExpandColumnIds []string           `json:"expandColumnIds,omitempty"`
	ExpandPaths     []string           `json:"expandPaths,omitempty"`
}

type QueryRowsResponse struct {
	Rows          []*Row `json:"rows,omitempty"`
	NextPageToken string `json:"nextPageToken,omitempty"`
	Count         int32  `json:"count,omitempty"`
}

type ExportRowsRequest struct {
	TableId   string         `json:"tableId,omitempty"`
	Format    string         `json:"format,omitempty"`
	Filter    map[string]any `json:"filter,omitempty"`
	ColumnIds []string       `json:"columnIds,omitempty"`
}

type ExportRowsResponse struct {
	Format  string `json:"format,omitempty"`
	Content string `json:"content,omitempty"`
}

type SearchRowsRequest struct {
	TableId   string         `json:"tableId,omitempty"`
	Query     string         `json:"query,omitempty"`
	Filter    map[string]any `json:"filter,omitempty"`
	PageSize  int32          `json:"pageSize,omitempty"`
	PageToken string         `json:"pageToken,omitempty"`
}

type SearchRowsResponse struct {
	Rows          []*Row `json:"rows,omitempty"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

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
