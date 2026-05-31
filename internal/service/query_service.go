package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/columntype"
	"github.com/solat/lowcode-database/internal/dsl"
	"github.com/solat/lowcode-database/internal/query"
)

// querySpec holds merged query parameters from datasource/request.
type querySpec struct {
	TableID         string
	Filter          map[string]any
	Sort            []*apiv1.SortOrder
	ColumnIds       []string
	PageSize        int32
	PageToken       string
	ExpandColumnIds []string
	ExpandPaths     []string
}

func (s *LowcodeService) QueryRows(ctx context.Context, req *apiv1.QueryRowsRequest) (*apiv1.QueryRowsResponse, error) {
	spec := querySpec{
		TableID:         req.TableId,
		Filter:          req.Filter,
		Sort:            req.Sort,
		ColumnIds:       req.ColumnIds,
		PageSize:        req.PageSize,
		PageToken:       req.PageToken,
		ExpandColumnIds: req.ExpandColumnIds,
		ExpandPaths:     req.ExpandPaths,
	}
	return s.executeQuery(ctx, spec)
}

func (s *LowcodeService) QueryDataSource(ctx context.Context, req *apiv1.QueryDataSourceRequest) (*apiv1.QueryDataSourceResponse, error) {
	start := time.Now()
	tid, tidErr := s.tenantID(ctx)

	ds, err := s.loadDataSourceSpec(ctx, req.DataSourceId)
	if err != nil {
		s.recordDataSourceQuery(ctx, tid, req.DataSourceId, start, err, 0, "")
		return nil, err
	}
	resp, err := s.executeQuery(ctx, querySpec{
		TableID:   ds.TableId,
		Filter:    mergeFilters(ds.Filter, req.Filter),
		Sort:      ds.Sort,
		ColumnIds: ds.ColumnIds,
		PageSize:  req.PageSize,
		PageToken: req.PageToken,
	})
	rowCount := int32(0)
	if resp != nil {
		rowCount = int32(len(resp.Rows))
	}
	if tidErr != nil {
		tid = ""
	}
	s.recordDataSourceQuery(ctx, tid, req.DataSourceId, start, err, rowCount, ds.TableId)
	if err != nil {
		return nil, err
	}
	return &apiv1.QueryDataSourceResponse{
		Rows:          resp.Rows,
		NextPageToken: resp.NextPageToken,
		Count:         resp.Count,
	}, nil
}

func (s *LowcodeService) recordDataSourceQuery(ctx context.Context, tenantID, dataSourceID string, start time.Time, err error, rowCount int32, tableID string) {
	if tenantID == "" || dataSourceID == "" {
		return
	}
	duration := time.Since(start)
	s.dsMetrics.Record(ctx, tenantID, dataSourceID, duration, err)

	if s.log == nil {
		return
	}
	attrs := []any{
		"tenant_id", tenantID,
		"datasource_id", dataSourceID,
		"duration_ms", duration.Milliseconds(),
		"row_count", rowCount,
	}
	if tableID != "" {
		attrs = append(attrs, "table_id", tableID)
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		s.log.Warn("datasource query failed", attrs...)
		return
	}
	if stats, statsErr := s.dsMetrics.Stats(ctx, tenantID, dataSourceID); statsErr == nil && stats.Count > 0 {
		attrs = append(attrs, "avg_duration_ms", stats.AvgDuration.Milliseconds(), "window_count", stats.Count)
	}
	if duration >= s.slowQueryThreshold {
		s.log.Warn("slow datasource query", attrs...)
	} else {
		s.log.Info("datasource query", attrs...)
	}
}

func mergeFilters(base, extra map[string]any) map[string]any {
	if len(base) == 0 {
		return extra
	}
	if len(extra) == 0 {
		return base
	}
	return map[string]any{
		"type": "AND",
		"val":  []any{base, extra},
	}
}

type loadedDataSource struct {
	TableId   string
	Filter    map[string]any
	Sort      []*apiv1.SortOrder
	ColumnIds []string
}

func (s *LowcodeService) loadDataSourceSpec(ctx context.Context, dsID string) (*loadedDataSource, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	key := cacheKeyDataSource(tid, dsID)
	if s.cache != nil {
		var cached loadedDataSource
		if ok, _ := s.cache.Get(ctx, key, &cached); ok {
			return &cached, nil
		}
	}

	meta := s.tenants.MetaPool()
	var tableID string
	var filter map[string]any
	var sortJSON []byte
	var colIDs []uuid.UUID
	if err := meta.QueryRow(ctx, `
		SELECT table_id, filter, sort, column_ids
		FROM lc_data_sources WHERE id = $1 AND tenant_id = $2`,
		dsID, tid,
	).Scan(&tableID, &filter, &sortJSON, &colIDs); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("data source not found")
		}
		return nil, err
	}
	var sort []*apiv1.SortOrder
	if len(sortJSON) > 0 {
		_ = json.Unmarshal(sortJSON, &sort)
	}
	ids := make([]string, len(colIDs))
	for i, id := range colIDs {
		ids[i] = id.String()
	}
	out := &loadedDataSource{TableId: tableID, Filter: filter, Sort: sort, ColumnIds: ids}
	if s.cache != nil {
		_ = s.cache.Set(ctx, key, out, s.cacheTTL)
	}
	return out, nil
}

func (s *LowcodeService) executeQuery(ctx context.Context, spec querySpec) (resp *apiv1.QueryRowsResponse, execErr error) {
	start := time.Now()
	defer func() {
		if execErr != nil {
			s.logQueryExecution(spec.TableID, start, 0, 0, execErr)
		}
	}()

	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := spec.TableID
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}

	allCols, schemaName, tableName, err := s.loadAllColumnMeta(ctx, tableID)
	if err != nil {
		return nil, err
	}
	physCols := filterPhysicalCols(allCols)
	if len(physCols) == 0 {
		return &apiv1.QueryRowsResponse{}, nil
	}

	selCols := physCols
	if len(spec.ColumnIds) > 0 {
		want := map[string]struct{}{}
		for _, id := range spec.ColumnIds {
			want[id] = struct{}{}
		}
		var subset []columnMeta
		for _, c := range physCols {
			if _, ok := want[c.Id]; ok {
				subset = append(subset, c)
			}
		}
		if len(subset) > 0 {
			selCols = subset
		}
	}

	pageSize := normalizePageSize(spec.PageSize, s.maxRow)

	lookupSpecs, err := s.buildLookupJoinSpecs(ctx, tableID)
	if err != nil {
		return nil, err
	}

	formulaSpecs, rollupSpecs, err := s.buildComputedSpecs(ctx, tableID, allCols)
	if err != nil {
		return nil, err
	}

	const baseAlias = "_b"
	columnSQL := baseAlias + ".id"
	qCols := make([]query.ColumnMeta, len(selCols))
	for i, c := range selCols {
		columnSQL += ", " + baseAlias + "." + c.PgColumn
		qCols[i] = query.ColumnMeta{ID: c.Id, PgColumn: c.PgColumn, PgType: c.PgType}
	}
	for _, lk := range lookupSpecs {
		columnSQL += ", " + pgx.Identifier{lk.Alias}.Sanitize() + "." + pgx.Identifier{lk.TargetPgCol}.Sanitize() +
			" AS " + pgx.Identifier{"lkval_" + lk.LookupColumnID}.Sanitize()
	}
	for _, f := range formulaSpecs {
		columnSQL += ", (" + f.SQL + ") AS " + pgx.Identifier{"f_" + f.ColumnID}.Sanitize()
	}
	for _, r := range rollupSpecs {
		columnSQL += ", (" + r.SQL + ") AS " + pgx.Identifier{"r_" + r.ColumnID}.Sanitize()
	}

	fromSQL := fmt.Sprintf(`%s.%s AS %s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		pgx.Identifier{baseAlias}.Sanitize(),
	)
	for _, lk := range lookupSpecs {
		fromSQL += fmt.Sprintf(` LEFT JOIN %s.%s AS %s ON %s.%s = %s.id`,
			pgx.Identifier{lk.TargetSchema}.Sanitize(),
			pgx.Identifier{lk.TargetTable}.Sanitize(),
			pgx.Identifier{lk.Alias}.Sanitize(),
			pgx.Identifier{baseAlias}.Sanitize(),
			pgx.Identifier{lk.BaseFKPgCol}.Sanitize(),
			pgx.Identifier{lk.Alias}.Sanitize(),
		)
	}

	attrMap := query.AttrMapFromColumns(baseAlias, qCols)
	for _, c := range physCols {
		attrMap[c.Id] = baseAlias + "." + c.PgColumn
		attrMap[c.Name] = baseAlias + "." + c.PgColumn
	}

	whereParts := []string{}
	args := []any{}
	argIdx := 1

	if spec.PageToken != "" {
		whereParts = append(whereParts, baseAlias+".id > $"+fmt.Sprint(argIdx))
		args = append(args, spec.PageToken)
		argIdx++
	}

	filterWhere, err := dsl.Parse(spec.Filter)
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}
	if filterWhere.Type != "" {
		wSQL, wArgs, err := query.BuildWhere(filterWhere, attrMap, argIdx)
		if err != nil {
			return nil, err
		}
		if wSQL != "" {
			whereParts = append(whereParts, wSQL)
			args = append(args, wArgs...)
			argIdx += len(wArgs)
		}
	}

	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = " WHERE " + strings.Join(whereParts, " AND ")
	}

	var orders []query.OrderSpec
	for _, o := range spec.Sort {
		orders = append(orders, query.OrderSpec{Attribute: o.Attribute, SortOrder: o.SortOrder})
	}
	orderSQL := query.BuildOrderBy(orders, attrMap, "id")
	orderClause := ""
	if orderSQL != "" {
		orderClause = " ORDER BY " + orderSQL
	}

	limitArg := argIdx
	args = append(args, pageSize+1)

	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s AS %s%s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		pgx.Identifier{baseAlias}.Sanitize(),
		whereSQL,
	)
	var total int32
	if err := data.QueryRow(ctx, countSQL, args[:len(args)-1]...).Scan(&total); err != nil {
		return nil, err
	}

	querySQL := fmt.Sprintf(`SELECT %s FROM %s%s%s LIMIT $%d`,
		columnSQL, fromSQL, whereSQL, orderClause, limitArg,
	)
	rows, err := data.Query(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out apiv1.QueryRowsResponse
	var lastID string
	for rows.Next() {
		nScan := 1 + len(selCols) + len(lookupSpecs) + len(formulaSpecs) + len(rollupSpecs)
		scanTargets := make([]any, nScan)
		var id string
		scanTargets[0] = &id
		values := make([]any, len(selCols))
		for i := range values {
			values[i] = new(any)
			scanTargets[i+1] = values[i]
		}
		off := 1 + len(selCols)
		lkVals := make([]*any, len(lookupSpecs))
		for i := range lookupSpecs {
			lkVals[i] = new(any)
			scanTargets[off+i] = lkVals[i]
		}
		off += len(lookupSpecs)
		formulaVals := make([]*any, len(formulaSpecs))
		for i := range formulaSpecs {
			formulaVals[i] = new(any)
			scanTargets[off+i] = formulaVals[i]
		}
		off += len(formulaSpecs)
		rollupVals := make([]*any, len(rollupSpecs))
		for i := range rollupSpecs {
			rollupVals[i] = new(any)
			scanTargets[off+i] = rollupVals[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, err
		}
		lastID = id

		row := &apiv1.Row{Id: id, Cells: make(map[string]*apiv1.Value)}
		for i, c := range selCols {
			vPtr := values[i].(*any)
			if *vPtr != nil {
				row.Cells[c.Id] = anyToValue(*vPtr)
			}
		}
		for i, lk := range lookupSpecs {
			if lkVals[i] != nil && *lkVals[i] != nil {
				row.Cells[lk.LookupColumnID] = anyToValue(*lkVals[i])
			} else {
				row.Cells[lk.LookupColumnID] = &apiv1.Value{JsonValue: json.RawMessage("null")}
			}
		}
		for i, f := range formulaSpecs {
			if formulaVals[i] != nil && *formulaVals[i] != nil {
				row.Cells[f.ColumnID] = anyToValue(*formulaVals[i])
			} else {
				row.Cells[f.ColumnID] = &apiv1.Value{JsonValue: json.RawMessage("null")}
			}
		}
		for i, r := range rollupSpecs {
			if rollupVals[i] != nil && *rollupVals[i] != nil {
				row.Cells[r.ColumnID] = anyToValue(*rollupVals[i])
			}
		}

		if len(spec.ExpandColumnIds) > 0 || len(spec.ExpandPaths) > 0 {
			if err := s.applyExpansions(ctx, tableID, row, spec.ExpandColumnIds, spec.ExpandPaths); err != nil {
				return nil, err
			}
		}

		out.Rows = append(out.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out.Count = total
	if int32(len(out.Rows)) > pageSize {
		out.Rows = out.Rows[:pageSize]
		out.NextPageToken = lastID
	}
	s.logQueryExecution(tableID, start, len(out.Rows), total, nil)
	return &out, nil
}

func (s *LowcodeService) logQueryExecution(tableID string, start time.Time, rowCount int, total int32, err error) {
	if s.log == nil {
		return
	}
	duration := time.Since(start)
	attrs := []any{
		"table_id", tableID,
		"duration_ms", duration.Milliseconds(),
		"row_count", rowCount,
		"total_count", total,
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		s.log.Warn("query failed", attrs...)
		return
	}
	if duration >= s.slowQueryThreshold {
		s.log.Warn("slow query", attrs...)
	}
}

func normalizePageSize(pageSize, maxRow int32) int32 {
	if pageSize <= 0 {
		if maxRow > 0 {
			pageSize = maxRow
		} else {
			pageSize = 50
		}
	}
	if maxRow > 0 && pageSize > maxRow {
		pageSize = maxRow
	} else if maxRow <= 0 && pageSize > 100 {
		pageSize = 100
	}
	return pageSize
}

type formulaSpec struct {
	ColumnID string
	SQL      string
}

type rollupSpec struct {
	ColumnID string
	SQL      string
}

func (s *LowcodeService) buildComputedSpecs(ctx context.Context, tableID string, allCols []fullColumnMeta) ([]formulaSpec, []rollupSpec, error) {
	const baseAlias = "_b"
	nameToPg := map[string]string{}
	for _, c := range allCols {
		if !c.IsVirtual {
			nameToPg[c.Name] = c.PgColumn
		}
	}
	var formulas []formulaSpec
	var rollups []rollupSpec
	for _, c := range allCols {
		switch c.Kind {
		case "formula":
			expr := formulaExpression(c.Config)
			if expr == "" {
				continue
			}
			sql, err := compileFormulaExpression(expr, baseAlias, nameToPg)
			if err != nil {
				return nil, nil, err
			}
			formulas = append(formulas, formulaSpec{ColumnID: c.Id, SQL: sql})
		case "rollup":
			relID := cfgString(c.Config, "relation_column_id")
			fieldID := cfgString(c.Config, "target_column_id")
			agg := cfgString(c.Config, "aggregate")
			if relID == "" {
				continue
			}
			rels, err := s.loadRelationshipColumns(ctx, tableID, []string{relID})
			if err != nil || len(rels) == 0 {
				continue
			}
			rel := rels[0]
			tgtCols, tgtSchema, tgtTable, err := s.loadColumns(ctx, rel.TargetTableId)
			if err != nil {
				continue
			}
			var linkPg, targetPg string
			tid, _ := s.tenantID(ctx)
			meta := s.tenants.MetaPool()
			if rel.LinkColumnId != "" {
				_ = meta.QueryRow(ctx, `SELECT pg_column FROM lc_columns WHERE id = $1 AND tenant_id = $2`, rel.LinkColumnId, tid).Scan(&linkPg)
			}
			for _, tc := range tgtCols {
				if tc.Id == fieldID {
					targetPg = tc.PgColumn
					break
				}
			}
			if linkPg == "" {
				continue
			}
			sql := rollupAggregateSQL(agg, targetPg, linkPg, tgtSchema, tgtTable, baseAlias)
			rollups = append(rollups, rollupSpec{ColumnID: c.Id, SQL: sql})
		}
	}
	return formulas, rollups, nil
}

type fullColumnMeta struct {
	columnMeta
	Kind      string
	Config    map[string]any
	IsVirtual bool
}

func (s *LowcodeService) loadAllColumnMeta(ctx context.Context, tableID string) ([]fullColumnMeta, string, string, error) {
	resolvedName, err := s.resolveTableName(ctx, tableID)
	if err != nil {
		return nil, "", "", err
	}
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, "", "", err
	}
	key := cacheKeyColumns(tid, resolvedName)
	if s.cache != nil {
		var cached cachedColumnMetaBundle
		if ok, _ := s.cache.Get(ctx, key, &cached); ok {
			return cached.Cols, cached.SchemaName, cached.TableName, nil
		}
	}

	meta := s.tenants.MetaPool()
	const q = `
		SELECT c.id, c.table_id, c.name, c.type_id, c.pg_column, c.is_nullable, c.position,
		       c.config, t.schema_name, t.table_name
		FROM lc_columns c
		JOIN lc_tables t ON c.table_id = t.name AND c.tenant_id = t.tenant_id
		WHERE c.table_id = $1 AND c.tenant_id = $2
		ORDER BY c.position
	`
	rows, err := meta.Query(ctx, q, resolvedName, tid)
	if err != nil {
		return nil, "", "", err
	}
	defer rows.Close()

	var out []fullColumnMeta
	var schemaName, tableName string
	for rows.Next() {
		var c fullColumnMeta
		if err := rows.Scan(&c.Id, &c.TableId, &c.Name, &c.TypeId, &c.PgColumn, &c.IsNullable, &c.Position,
			&c.Config, &schemaName, &tableName); err != nil {
			return nil, "", "", err
		}
		c.PgType = s.columnPgTypeSQL(ctx, tid, c.TypeId, c.Config)
		c.Kind = columntype.Kind(c.TypeId)
		c.IsVirtual = columntype.IsVirtual(c.TypeId)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", "", err
	}
	if s.cache != nil {
		_ = s.cache.Set(ctx, key, cachedColumnMetaBundle{
			Cols: out, SchemaName: schemaName, TableName: tableName,
		}, s.cacheTTL)
	}
	return out, schemaName, tableName, nil
}

func filterPhysicalCols(all []fullColumnMeta) []columnMeta {
	var out []columnMeta
	for _, c := range all {
		if !c.IsVirtual {
			out = append(out, c.columnMeta)
		}
	}
	return out
}

func (s *LowcodeService) applyExpansions(ctx context.Context, tableID string, row *apiv1.Row, expandIDs, expandPaths []string) error {
	if len(expandIDs) > 0 {
		relCols, err := s.loadRelationshipColumns(ctx, tableID, expandIDs)
		if err != nil {
			return err
		}
		for _, rel := range relCols {
			related, err := s.fetchRelatedRows(ctx, rel, row.Id, row.Cells, fetchRelatedOpts{})
			if err != nil {
				return err
			}
			row.Cells[rel.Id] = relationshipExpandValue(rel, related)
		}
	}
	for _, path := range expandPaths {
		parts := strings.Split(path, ".")
		var nonEmpty []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				nonEmpty = append(nonEmpty, p)
			}
		}
		if len(nonEmpty) < 2 {
			return fmt.Errorf("expand path %q needs at least two segments", path)
		}
		v, err := s.expandPathResult(ctx, tableID, row.Id, row.Cells, nonEmpty, 0)
		if err != nil {
			return fmt.Errorf("expand path %q: %w", path, err)
		}
		row.Cells[path] = apiv1.JsonValue(v)
	}
	return nil
}
