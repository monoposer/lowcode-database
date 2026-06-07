package query

import (
	"time"

	"github.com/monoposer/lowcode-database/internal/apiv1"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
)

// Query is a saved projection + filter + sort (meta lc_queries).
type Query struct {
	Id        string             `json:"id,omitempty"`
	Name      string             `json:"name,omitempty"`
	Label     string             `json:"label,omitempty"`
	TableId   string             `json:"tableId,omitempty"`
	Filter    map[string]any     `json:"filter,omitempty"`
	Sort      []*apiv1.SortOrder `json:"sort,omitempty"`
	ColumnIds []string           `json:"columnIds,omitempty"`
	Config    map[string]any     `json:"config,omitempty"`
	CreatedAt time.Time          `json:"createdAt,omitempty"`
	UpdatedAt time.Time          `json:"updatedAt,omitempty"`
}

type CreateQueryRequest struct {
	Name      string             `json:"name,omitempty"`
	Label     string             `json:"label,omitempty"`
	TableId   string             `json:"tableId,omitempty"`
	Filter    map[string]any     `json:"filter,omitempty"`
	Sort      []*apiv1.SortOrder `json:"sort,omitempty"`
	ColumnIds []string           `json:"columnIds,omitempty"`
	Config    map[string]any     `json:"config,omitempty"`
}

type CreateQueryResponse struct {
	Query *Query `json:"query,omitempty"`
}

type ListQueriesRequest struct {
	TableId string `json:"tableId,omitempty"`
}

type ListQueriesResponse struct {
	Queries []*Query `json:"queries,omitempty"`
}

type GetQueryRequest struct {
	TableId string `json:"tableId,omitempty"`
	Name    string `json:"name,omitempty"`
}

type GetQueryResponse struct {
	Query *Query `json:"query,omitempty"`
}

type UpdateQueryRequest struct {
	TableId   string             `json:"tableId,omitempty"`
	Name      string             `json:"name,omitempty"`
	Label     string             `json:"label,omitempty"`
	Filter    map[string]any     `json:"filter,omitempty"`
	Sort      []*apiv1.SortOrder `json:"sort,omitempty"`
	ColumnIds []string           `json:"columnIds,omitempty"`
	Config    map[string]any     `json:"config,omitempty"`
}

type UpdateQueryResponse struct {
	Query *Query `json:"query,omitempty"`
}

type DeleteQueryRequest struct {
	TableId string `json:"tableId,omitempty"`
	Name    string `json:"name,omitempty"`
}

type DeleteQueryResponse struct{}

type ExecuteQueryRequest struct {
	TableId     string         `json:"tableId,omitempty"`
	QueryId     string         `json:"queryId,omitempty"`
	PageSize    int32          `json:"pageSize,omitempty"`
	PageToken   string         `json:"pageToken,omitempty"`
	Filter      map[string]any `json:"filter,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	ColumnIds   []string       `json:"columnIds,omitempty"`
	Consistency string         `json:"consistency,omitempty"`
}

type ExecuteQueryResponse struct {
	Rows          []*row.Row `json:"rows,omitempty"`
	NextPageToken string     `json:"nextPageToken,omitempty"`
	Count         int32      `json:"count,omitempty"`
}
