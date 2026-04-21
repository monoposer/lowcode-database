package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
)

// -------- DataSource (list / view definition) --------

func (s *LowcodeService) CreateDataSource(ctx context.Context, req *apiv1.CreateDataSourceRequest) (*apiv1.CreateDataSourceResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.TableId == "" {
		return nil, fmt.Errorf("table_id is required")
	}
	tableName, err := s.resolveTableName(ctx, req.TableId)
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

	meta := s.tenants.MetaPool()
	row := meta.QueryRow(ctx, `
		INSERT INTO lc_data_sources (tenant_id, name, label, table_id, filter, sort, column_ids, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, name, label, table_id, filter, sort, column_ids, config, created_at, updated_at`,
		tid, req.Name, req.Label, tableName, filter, sortJSON, parseColumnUUIDs(req.ColumnIds), cfg)
	ds, err := scanDataSourceRow(row)
	if err != nil {
		return nil, err
	}
	return &apiv1.CreateDataSourceResponse{DataSource: ds}, nil
}

func (s *LowcodeService) ListDataSources(ctx context.Context, req *apiv1.ListDataSourcesRequest) (*apiv1.ListDataSourcesResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	var rows pgx.Rows
	if req.TableId != "" {
		tableName, err := s.resolveTableName(ctx, req.TableId)
		if err != nil {
			return nil, err
		}
		rows, err = meta.Query(ctx, `
			SELECT id, name, label, table_id, filter, sort, column_ids, config, created_at, updated_at
			FROM lc_data_sources WHERE tenant_id = $1 AND table_id = $2 ORDER BY name`, tid, tableName)
		if err != nil {
			return nil, err
		}
	} else {
		rows, err = meta.Query(ctx, `
			SELECT id, name, label, table_id, filter, sort, column_ids, config, created_at, updated_at
			FROM lc_data_sources WHERE tenant_id = $1 ORDER BY name`, tid)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var resp apiv1.ListDataSourcesResponse
	for rows.Next() {
		ds, err := scanDataSource(rows)
		if err != nil {
			return nil, err
		}
		resp.DataSources = append(resp.DataSources, ds)
	}
	return &resp, rows.Err()
}

func (s *LowcodeService) GetDataSource(ctx context.Context, req *apiv1.GetDataSourceRequest) (*apiv1.GetDataSourceResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	row := s.tenants.MetaPool().QueryRow(ctx, `
		SELECT id, name, label, table_id, filter, sort, column_ids, config, created_at, updated_at
		FROM lc_data_sources WHERE id = $1 AND tenant_id = $2`, req.Id, tid)
	ds, err := scanDataSourceRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("data source not found")
		}
		return nil, err
	}
	return &apiv1.GetDataSourceResponse{DataSource: ds}, nil
}

func (s *LowcodeService) UpdateDataSource(ctx context.Context, req *apiv1.UpdateDataSourceRequest) (*apiv1.UpdateDataSourceResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	var sortJSON []byte
	if req.Sort != nil {
		sortJSON, _ = json.Marshal(req.Sort)
	}
	var colArg any
	if req.ColumnIds != nil {
		colArg = parseColumnUUIDs(req.ColumnIds)
	}
	row := s.tenants.MetaPool().QueryRow(ctx, `
		UPDATE lc_data_sources SET
		  label = COALESCE(NULLIF($2,''), label),
		  filter = COALESCE($3, filter),
		  sort = COALESCE($4, sort),
		  column_ids = COALESCE($5, column_ids),
		  config = COALESCE($6, config),
		  updated_at = now()
		WHERE id = $1 AND tenant_id = $7
		RETURNING id, name, label, table_id, filter, sort, column_ids, config, created_at, updated_at`,
		req.Id, req.Label, req.Filter, nullJSON(sortJSON), colArg, req.Config, tid)
	ds, err := scanDataSourceRow(row)
	if err != nil {
		return nil, err
	}
	s.invalidateDataSourceCache(ctx, ds.Id)
	return &apiv1.UpdateDataSourceResponse{DataSource: ds}, nil
}

func (s *LowcodeService) DeleteDataSource(ctx context.Context, req *apiv1.DeleteDataSourceRequest) (*apiv1.DeleteDataSourceResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	_, err = s.tenants.MetaPool().Exec(ctx, `DELETE FROM lc_data_sources WHERE id = $1 AND tenant_id = $2`, req.Id, tid)
	if err == nil {
		s.invalidateDataSourceCache(ctx, req.Id)
	}
	return &apiv1.DeleteDataSourceResponse{}, err
}

func scanDataSource(rows pgx.Rows) (*apiv1.DataSource, error) {
	var ds apiv1.DataSource
	var filter map[string]any
	var sortJSON []byte
	var colIDs []uuid.UUID
	var cfg map[string]any
	var createdAt, updatedAt time.Time
	if err := rows.Scan(&ds.Id, &ds.Name, &ds.Label, &ds.TableId, &filter, &sortJSON, &colIDs, &cfg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	ds.Filter = filter
	ds.Config = cfg
	ds.ColumnIds = columnUUIDsToStrings(colIDs)
	_ = json.Unmarshal(sortJSON, &ds.Sort)
	ds.CreatedAt = createdAt
	ds.UpdatedAt = updatedAt
	return &ds, nil
}

func scanDataSourceRow(row pgx.Row) (*apiv1.DataSource, error) {
	var ds apiv1.DataSource
	var filter map[string]any
	var sortJSON []byte
	var colIDs []uuid.UUID
	var cfg map[string]any
	var createdAt, updatedAt time.Time
	if err := row.Scan(&ds.Id, &ds.Name, &ds.Label, &ds.TableId, &filter, &sortJSON, &colIDs, &cfg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	ds.Filter = filter
	ds.Config = cfg
	ds.ColumnIds = columnUUIDsToStrings(colIDs)
	_ = json.Unmarshal(sortJSON, &ds.Sort)
	ds.CreatedAt = createdAt
	ds.UpdatedAt = updatedAt
	return &ds, nil
}

func parseColumnUUIDs(ids []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if u, err := uuid.Parse(id); err == nil {
			out = append(out, u)
		}
	}
	return out
}

func columnUUIDsToStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
