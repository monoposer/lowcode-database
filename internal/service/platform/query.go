package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/monoposer/lowcode-database/internal/apiv1/query"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func scanQuery(rows pgx.Rows) (*query.Query, error) {
	var ds query.Query
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

func scanQueryRow(row pgx.Row) (*query.Query, error) {
	var ds query.Query
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

func (s *Platform) ListQueries(ctx context.Context, req *query.ListQueriesRequest) (*query.ListQueriesResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
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
			FROM lc_queries WHERE tenant_id = $1 AND base_id = $2 AND table_id = $3 ORDER BY name`, tid, baseID, tableName)
		if err != nil {
			return nil, err
		}
	} else {
		rows, err = meta.Query(ctx, `
			SELECT table_id, name, label, filter, sort, column_names, config, created_at, updated_at
			FROM lc_queries WHERE tenant_id = $1 AND base_id = $2 ORDER BY table_id, name`, tid, baseID)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var resp query.ListQueriesResponse
	for rows.Next() {
		ds, err := scanQuery(rows)
		if err != nil {
			return nil, err
		}
		resp.Queries = append(resp.Queries, ds)
	}
	return &resp, rows.Err()
}

func (s *Platform) GetQuery(ctx context.Context, req *query.GetQueryRequest) (*query.GetQueryResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	tableID, dsName, err := s.ResolveQueryRef(ctx, req.TableId, req.Name)
	if err != nil {
		return nil, err
	}
	row := s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT table_id, name, label, filter, sort, column_names, config, created_at, updated_at
		FROM lc_queries WHERE tenant_id = $1 AND base_id = $2 AND table_id = $3 AND name = $4`, tid, baseID, tableID, dsName)
	ds, err := scanQueryRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("query not found")
		}
		return nil, err
	}
	return &query.GetQueryResponse{Query: ds}, nil
}

func (s *Platform) ResolveQueryRef(ctx context.Context, tableRef, dsRef string) (tableID, dsName string, err error) {
	return s.meta().ResolveQueryRef(ctx, tableRef, dsRef)
}

func (s *Platform) CreateQuery(ctx context.Context, req *query.CreateQueryRequest) (*query.CreateQueryResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
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
		INSERT INTO lc_queries (tenant_id, base_id, table_id, name, label, filter, sort, column_names, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING table_id, name, label, filter, sort, column_names, config, created_at, updated_at`,
		tid, baseID, tableName, req.Name, req.Label, filter, sortJSON, colNames, cfg)
	ds, err := scanQueryRow(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("query %q already exists for this table", req.Name)
		}
		return nil, err
	}
	return &query.CreateQueryResponse{Query: ds}, nil
}

func (s *Platform) UpdateQuery(ctx context.Context, req *query.UpdateQueryRequest) (*query.UpdateQueryResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	tableID, dsName, err := s.ResolveQueryRef(ctx, req.TableId, req.Name)
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
		UPDATE lc_queries SET
		  label = COALESCE(NULLIF($5,''), label),
		  filter = COALESCE($6, filter),
		  sort = COALESCE($7, sort),
		  column_names = COALESCE($8, column_names),
		  config = COALESCE($9, config),
		  updated_at = now()
		WHERE tenant_id = $1 AND base_id = $2 AND table_id = $3 AND name = $4
		RETURNING table_id, name, label, filter, sort, column_names, config, created_at, updated_at`,
		tid, baseID, tableID, dsName, req.Label, req.Filter, shared.NullJSON(sortJSON), colArg, req.Config)
	ds, err := scanQueryRow(row)
	if err != nil {
		return nil, err
	}
	s.B.InvalidateQueryCache(ctx, ds.TableId, ds.Name)
	return &query.UpdateQueryResponse{Query: ds}, nil
}

func (s *Platform) DeleteQuery(ctx context.Context, req *query.DeleteQueryRequest) (*query.DeleteQueryResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	tableID, dsName, err := s.ResolveQueryRef(ctx, req.TableId, req.Name)
	if err != nil {
		return nil, err
	}
	_, err = s.B.Tenants.MetaPool().Exec(ctx, `
		DELETE FROM lc_queries WHERE tenant_id = $1 AND base_id = $2 AND table_id = $3 AND name = $4`,
		tid, baseID, tableID, dsName)
	if err == nil {
		s.B.InvalidateQueryCache(ctx, tableID, dsName)
	}
	return &query.DeleteQueryResponse{}, err
}
