package datasource

import (
	"time"

	"github.com/monoposer/lowcode-database/internal/apiv1"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
)

type DataSource struct {
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

type CreateDataSourceRequest struct {
	Name      string             `json:"name,omitempty"`
	Label     string             `json:"label,omitempty"`
	TableId   string             `json:"tableId,omitempty"`
	Filter    map[string]any     `json:"filter,omitempty"`
	Sort      []*apiv1.SortOrder `json:"sort,omitempty"`
	ColumnIds []string           `json:"columnIds,omitempty"`
	Config    map[string]any     `json:"config,omitempty"`
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
	Name    string `json:"name,omitempty"`
}

type GetDataSourceResponse struct {
	DataSource *DataSource `json:"dataSource,omitempty"`
}

type UpdateDataSourceRequest struct {
	TableId   string             `json:"tableId,omitempty"`
	Name      string             `json:"name,omitempty"`
	Label     string             `json:"label,omitempty"`
	Filter    map[string]any     `json:"filter,omitempty"`
	Sort      []*apiv1.SortOrder `json:"sort,omitempty"`
	ColumnIds []string           `json:"columnIds,omitempty"`
	Config    map[string]any     `json:"config,omitempty"`
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
	DataSourceId string         `json:"dataSourceId,omitempty"`
	PageSize     int32          `json:"pageSize,omitempty"`
	PageToken    string         `json:"pageToken,omitempty"`
	Filter       map[string]any `json:"filter,omitempty"`
	Params       map[string]any `json:"params,omitempty"`
	ColumnIds    []string       `json:"columnIds,omitempty"`
}

type QueryDataSourceResponse struct {
	Rows          []*row.Row `json:"rows,omitempty"`
	NextPageToken string     `json:"nextPageToken,omitempty"`
	Count         int32      `json:"count,omitempty"`
}
