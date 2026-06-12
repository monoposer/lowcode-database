package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/monoposer/lowcode-database/internal/apiv1/datasource"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"time"
)

func scanDataSource(rows pgx.Rows) (*datasource.DataSource, error) {
	var ds datasource.DataSource
	var filter map[string]any
	var sortJSON []byte
	var colNames []string
	var cfg map[string]any
	var createdAt, updatedAt time.Time
	if err := rows.Scan(&ds.TableId, &ds.Name, &ds.Label, &filter, &sortJSON, &colNames, &cfg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	ds.Id = ds.Name
	ds.Filter = filter
	ds.Config = cfg
	ds.ColumnIds = colNames
	_ = json.Unmarshal(sortJSON, &ds.Sort)
	ds.CreatedAt = createdAt
	ds.UpdatedAt = updatedAt
	return &ds, nil
}

func scanDataSourceRow(row pgx.Row) (*datasource.DataSource, error) {
	var ds datasource.DataSource
	var filter map[string]any
	var sortJSON []byte
	var colNames []string
	var cfg map[string]any
	var createdAt, updatedAt time.Time
	if err := row.Scan(&ds.TableId, &ds.Name, &ds.Label, &filter, &sortJSON, &colNames, &cfg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	ds.Id = ds.Name
	ds.Filter = filter
	ds.Config = cfg
	ds.ColumnIds = colNames
	_ = json.Unmarshal(sortJSON, &ds.Sort)
	ds.CreatedAt = createdAt
	ds.UpdatedAt = updatedAt
	return &ds, nil
}

func (s *Platform) ListDataSources(ctx context.Context, req *datasource.ListDataSourcesRequest) (*datasource.ListDataSourcesResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	var rows pgx.Rows
	if req.TableId != "" {
		tableName, err := s.B.ResolveTableName(ctx, req.TableId)
		if err != nil {
			return nil, err
		}
		rows, err = meta.Query(ctx, `
			SELECT table_id, name, label, filter, sort, column_names, config, created_at, updated_at
			FROM lc_data_sources WHERE tenant_id = $1 AND table_id = $2 ORDER BY name`, tid, tableName)
		if err != nil {
			return nil, err
		}
	} else {
		rows, err = meta.Query(ctx, `
			SELECT table_id, name, label, filter, sort, column_names, config, created_at, updated_at
			FROM lc_data_sources WHERE tenant_id = $1 ORDER BY table_id, name`, tid)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var resp datasource.ListDataSourcesResponse
	for rows.Next() {
		ds, err := scanDataSource(rows)
		if err != nil {
			return nil, err
		}
		resp.DataSources = append(resp.DataSources, ds)
	}
	return &resp, rows.Err()
}

func (s *Platform) GetDataSource(ctx context.Context, req *datasource.GetDataSourceRequest) (*datasource.GetDataSourceResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	tableID, dsName, err := s.ResolveDataSourceRef(ctx, req.TableId, req.Name)
	if err != nil {
		return nil, err
	}
	row := s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT table_id, name, label, filter, sort, column_names, config, created_at, updated_at
		FROM lc_data_sources WHERE tenant_id = $1 AND table_id = $2 AND name = $3`, tid, tableID, dsName)
	ds, err := scanDataSourceRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("data source not found")
		}
		return nil, err
	}
	return &datasource.GetDataSourceResponse{DataSource: ds}, nil
}

func (s *Platform) ResolveDataSourceRef(ctx context.Context, tableRef, dsRef string) (tableID, dsName string, err error) {
	return s.meta().ResolveDataSourceRef(ctx, tableRef, dsRef)
}

func (s *Platform) CreateDataSource(ctx context.Context, req *datasource.CreateDataSourceRequest) (*datasource.CreateDataSourceResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.TableId == "" {
		return nil, fmt.Errorf("table_id is required")
	}
	if err := shared.ValidateTableName(req.Name); err != nil {
		return nil, err
	}
	tableName, err := s.B.ResolveTableName(ctx, req.TableId)
	if err != nil {
		return nil, err
	}
	colNames, err := s.meta().NormalizeColumnNames(ctx, tid, tableName, req.ColumnIds)
	if err != nil {
		return nil, err
	}
	filter := req.Filter
	if filter == nil {
		filter = map[string]any{}
	}
	sortJSON, _ := json.Marshal(req.Sort)
	cfg := req.Config
	if cfg == nil {
		cfg = map[string]any{}
	}

	meta := s.B.Tenants.MetaPool()
	row := meta.QueryRow(ctx, `
		INSERT INTO lc_data_sources (tenant_id, table_id, name, label, filter, sort, column_names, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING table_id, name, label, filter, sort, column_names, config, created_at, updated_at`,
		tid, tableName, req.Name, req.Label, filter, sortJSON, colNames, cfg)
	ds, err := scanDataSourceRow(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("data source %q already exists for this table", req.Name)
		}
		return nil, err
	}
	return &datasource.CreateDataSourceResponse{DataSource: ds}, nil
}

func (s *Platform) UpdateDataSource(ctx context.Context, req *datasource.UpdateDataSourceRequest) (*datasource.UpdateDataSourceResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	tableID, dsName, err := s.ResolveDataSourceRef(ctx, req.TableId, req.Name)
	if err != nil {
		return nil, err
	}
	var sortJSON []byte
	if req.Sort != nil {
		sortJSON, _ = json.Marshal(req.Sort)
	}
	var colArg any
	if req.ColumnIds != nil {
		names, err := s.meta().NormalizeColumnNames(ctx, tid, tableID, req.ColumnIds)
		if err != nil {
			return nil, err
		}
		colArg = names
	}
	row := s.B.Tenants.MetaPool().QueryRow(ctx, `
		UPDATE lc_data_sources SET
		  label = COALESCE(NULLIF($4,''), label),
		  filter = COALESCE($5, filter),
		  sort = COALESCE($6, sort),
		  column_names = COALESCE($7, column_names),
		  config = COALESCE($8, config),
		  updated_at = now()
		WHERE tenant_id = $1 AND table_id = $2 AND name = $3
		RETURNING table_id, name, label, filter, sort, column_names, config, created_at, updated_at`,
		tid, tableID, dsName, req.Label, req.Filter, shared.NullJSON(sortJSON), colArg, req.Config)
	ds, err := scanDataSourceRow(row)
	if err != nil {
		return nil, err
	}
	s.B.InvalidateDataSourceCache(ctx, ds.TableId, ds.Name)
	return &datasource.UpdateDataSourceResponse{DataSource: ds}, nil
}

func (s *Platform) DeleteDataSource(ctx context.Context, req *datasource.DeleteDataSourceRequest) (*datasource.DeleteDataSourceResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	tableID, dsName, err := s.ResolveDataSourceRef(ctx, req.TableId, req.Name)
	if err != nil {
		return nil, err
	}
	_, err = s.B.Tenants.MetaPool().Exec(ctx, `
		DELETE FROM lc_data_sources WHERE tenant_id = $1 AND table_id = $2 AND name = $3`,
		tid, tableID, dsName)
	if err == nil {
		s.B.InvalidateDataSourceCache(ctx, tableID, dsName)
	}
	return &datasource.DeleteDataSourceResponse{}, err
}
