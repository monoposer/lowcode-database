package data

import (
	"github.com/monoposer/lowcode-database/internal/service/platform"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/dsl"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"github.com/monoposer/lowcode-database/pkg/tenant"
	"strings"
	"time"
)

func (s *Data) QueryRows(ctx context.Context, req *QueryRowsRequest) (*QueryRowsResponse, error) {
	ctx = withReadConsistency(ctx, req.Consistency)
	spec := querySpec{
		TableName:   req.TableName,
		Filter:    req.Filter,
		Sort:      req.Sort,
		ColumnIds: req.ColumnIds,
		PageSize:  req.PageSize,
		PageToken: req.PageToken,
	}
	return s.executeQuery(ctx, spec)
}

func (s *Data) ExecuteQuery(ctx context.Context, req *platform.ExecuteQueryRequest) (*QueryRowsResponse, error) {
	start := time.Now()
	ctx = withReadConsistency(ctx, req.Consistency)
	tid, tidErr := s.B.TenantID(ctx)

	ds, err := s.loadQuerySpec(ctx, req.TableName, req.QueryId)
	if err != nil {
		s.recordSavedQuery(ctx, tid, req.TableName, req.QueryId, start, err, 0, "")
		return nil, err
	}
	baseFilter := ds.Filter
	if len(req.Params) > 0 {
		baseFilter, err = dsl.SubstituteParams(ds.Filter, req.Params)
		if err != nil {
			s.recordSavedQuery(ctx, tid, req.TableName, req.QueryId, start, err, 0, "")
			return nil, err
		}
	}
	// Saved query = stored filter/sort/projection; response columns = projection (optionally narrowed by req.ColumnIds).
	colIds, err := s.resolveQueryProjection(ctx, ds.TableName, ds.ColumnIds, req.ColumnIds)
	if err != nil {
		s.recordSavedQuery(ctx, tid, req.TableName, req.QueryId, start, err, 0, "")
		return nil, err
	}
	if len(req.ColumnIds) > 0 && len(colIds) == 0 {
		err := fmt.Errorf("no columns match query projection")
		s.recordSavedQuery(ctx, tid, req.TableName, req.QueryId, start, err, 0, "")
		return nil, err
	}
	resp, err := s.executeQuery(ctx, querySpec{
		TableName:        ds.TableName,
		Filter:         mergeFilters(baseFilter, req.Filter),
		Sort:           ds.Sort,
		ColumnIds:      colIds,
		ColumnRestrict: true,
		PageSize:       req.PageSize,
		PageToken:      req.PageToken,
	})
	rowCount := int32(0)
	if resp != nil {
		rowCount = int32(len(resp.Rows))
	}
	if tidErr != nil {
		tid = ""
	}
	s.recordSavedQuery(ctx, tid, ds.TableName, req.QueryId, start, err, rowCount, ds.TableName)
	return resp, err
}

func (s *Data) loadQuerySpec(ctx context.Context, tableRef, dsName string) (*loadedQuery, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	tableName, name, err := s.meta().ResolveQueryRef(ctx, tableRef, dsName)
	if err != nil {
		return nil, err
	}
	key := shared.CacheKeyQuery(tid, tableName, name)
	if s.B.Cache != nil {
		var cached loadedQuery
		if ok, _ := s.B.Cache.Get(ctx, key, &cached); ok {
			return &cached, nil
		}
	}

	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	var filter map[string]any
	var sortJSON []byte
	var colNames []string
	if err := meta.QueryRow(ctx, `
		SELECT filter, sort, column_names
		FROM lc_queries WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3 AND name = $4`,
		tid, baseID, tableName, name,
	).Scan(&filter, &sortJSON, &colNames); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("query not found")
		}
		return nil, err
	}
	var sort []*shared.SortOrder
	if len(sortJSON) > 0 {
		_ = json.Unmarshal(sortJSON, &sort)
	}
	out := &loadedQuery{TableName: tableName, Filter: filter, Sort: sort, ColumnIds: colNames}
	_, _ = dsl.ParseCached(filter)
	if s.B.Cache != nil {
		_ = s.B.Cache.Set(ctx, key, out, s.B.CacheTTL)
	}
	return out, nil
}

func (s *Data) SearchRows(ctx context.Context, req *SearchRowsRequest) (*SearchRowsResponse, error) {
	ctx = withReadConsistency(ctx, req.Consistency)
	q := strings.TrimSpace(req.Query)
	if req.TableName == "" {
		return nil, fmt.Errorf("table_name is required")
	}
	if q == "" {
		return nil, fmt.Errorf("query is required")
	}
	filter := map[string]any{"field": "_fulltext_text", "op": "fts", "value": q}
	if req.Filter != nil {
		filter = map[string]any{"op": "and", "children": []any{filter, req.Filter}}
	}
	qresp, err := s.QueryRows(ctx, &QueryRowsRequest{
		TableName: req.TableName, Filter: filter, PageSize: req.PageSize, PageToken: req.PageToken,
	})
	if err != nil {
		return nil, err
	}
	return &SearchRowsResponse{Rows: qresp.Rows, NextPageToken: qresp.NextPageToken}, nil
}

func withReadConsistency(ctx context.Context, consistency string) context.Context {
	switch strings.ToLower(strings.TrimSpace(consistency)) {
	case "strong", "primary", "write":
		return tenant.WithStrongRead(ctx, true)
	default:
		return ctx
	}
}
