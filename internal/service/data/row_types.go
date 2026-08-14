package data

import "github.com/monoposer/lowcode-database/internal/service/shared"

type Row struct {
	Id      string                   `json:"id,omitempty"`
	Version int64                    `json:"version,omitempty"`
	Pending bool                     `json:"pending,omitempty"`
	Cells   map[string]*shared.Value `json:"cells,omitempty"`
}

type CreateRowRequest struct {
	TableName string                   `json:"tableName,omitempty"`
	Cells   map[string]*shared.Value `json:"cells,omitempty"`
}

type CreateRowResponse struct {
	Row *Row `json:"row,omitempty"`
}

type UpdateRowRequest struct {
	TableName string                   `json:"tableName,omitempty"`
	RowId   string                   `json:"rowId,omitempty"`
	Cells   map[string]*shared.Value `json:"cells,omitempty"`
}

type UpdateRowResponse struct {
	Row *Row `json:"row,omitempty"`
}

type DeleteRowRequest struct {
	TableName string `json:"tableName,omitempty"`
	RowId   string `json:"rowId,omitempty"`
}

type DeleteRowResponse struct{}

type GetRowRequest struct {
	TableName     string `json:"tableName,omitempty"`
	RowId       string `json:"rowId,omitempty"`
	Consistency string `json:"consistency,omitempty"`
}

type GetRowResponse struct {
	Row *Row `json:"row,omitempty"`
}

type ListRowsRequest struct {
	TableName     string `json:"tableName,omitempty"`
	PageSize    int32  `json:"pageSize,omitempty"`
	PageToken   string `json:"pageToken,omitempty"`
	Consistency string `json:"consistency,omitempty"`
}

type ListRowsResponse struct {
	Rows          []*Row `json:"rows,omitempty"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

type BulkUpsertRowItem struct {
	RowId string                   `json:"rowId,omitempty"`
	Cells map[string]*shared.Value `json:"cells,omitempty"`
}

type BulkUpsertRowsRequest struct {
	TableName string               `json:"tableName,omitempty"`
	Items   []*BulkUpsertRowItem `json:"items,omitempty"`
}

type BulkUpsertRowsResponse struct {
	Rows []*Row `json:"rows,omitempty"`
}

type BulkDeleteRowsRequest struct {
	TableName string   `json:"tableName,omitempty"`
	RowIds  []string `json:"rowIds,omitempty"`
}

type BulkDeleteRowsResponse struct{}

type QueryRowsRequest struct {
	TableName     string              `json:"tableName,omitempty"`
	Filter      map[string]any      `json:"filter,omitempty"`
	Sort        []*shared.SortOrder `json:"sort,omitempty"`
	ColumnIds   []string            `json:"columnIds,omitempty"`
	PageSize    int32               `json:"pageSize,omitempty"`
	PageToken   string              `json:"pageToken,omitempty"`
	Consistency string              `json:"consistency,omitempty"`
}

type QueryRowsResponse struct {
	Rows          []*Row `json:"rows,omitempty"`
	NextPageToken string `json:"nextPageToken,omitempty"`
	Count         int32  `json:"count,omitempty"`
}

type ExportRowsRequest struct {
	TableName     string         `json:"tableName,omitempty"`
	Format      string         `json:"format,omitempty"`
	Filter      map[string]any `json:"filter,omitempty"`
	ColumnIds   []string       `json:"columnIds,omitempty"`
	Consistency string         `json:"consistency,omitempty"`
}

type ExportRowsResponse struct {
	Format  string `json:"format,omitempty"`
	Content string `json:"content,omitempty"`
}

type SearchRowsRequest struct {
	TableName     string         `json:"tableName,omitempty"`
	Query       string         `json:"query,omitempty"`
	Filter      map[string]any `json:"filter,omitempty"`
	PageSize    int32          `json:"pageSize,omitempty"`
	PageToken   string         `json:"pageToken,omitempty"`
	Consistency string         `json:"consistency,omitempty"`
}

type SearchRowsResponse struct {
	Rows          []*Row `json:"rows,omitempty"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

func RowToMap(r *Row) map[string]any {
	if r == nil {
		return nil
	}
	m := map[string]any{"id": r.Id}
	for k, v := range r.Cells {
		m[k] = shared.ValueToNative(v)
	}
	return m
}
