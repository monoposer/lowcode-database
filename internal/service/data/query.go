package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/dsl"
	formulacompile "github.com/solat/lowcode-database/internal/formula"
	"github.com/solat/lowcode-database/internal/logger"
	"github.com/solat/lowcode-database/internal/query"
	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/platform"
	"github.com/solat/lowcode-database/internal/service/schema"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// querySpec holds merged query parameters from datasource/request.
type querySpec struct {
	TableID         string
	Filter          map[string]any
	Sort            []*apiv1.SortOrder
	ColumnIds       []string
	ColumnRestrict  bool // when true, ColumnIds limits output (empty = id only)
	PageSize        int32
	PageToken       string
	ExpandColumnIds []string
	ExpandPaths     []string
}

func (s *Data) QueryRows(ctx context.Context, req *apiv1.QueryRowsRequest) (*apiv1.QueryRowsResponse, error) {
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

func (s *Data) QueryDataSource(ctx context.Context, req *apiv1.QueryDataSourceRequest) (*apiv1.QueryDataSourceResponse, error) {
	start := time.Now()
	tid, tidErr := s.B.TenantID(ctx)

	ds, err := s.loadDataSourceSpec(ctx, req.TableId, req.DataSourceId)
	if err != nil {
		s.recordDataSourceQuery(ctx, tid, req.TableId, req.DataSourceId, start, err, 0, "")
		return nil, err
	}
	baseFilter := ds.Filter
	if len(req.Params) > 0 {
		baseFilter, err = dsl.SubstituteParams(ds.Filter, req.Params)
		if err != nil {
			s.recordDataSourceQuery(ctx, tid, req.TableId, req.DataSourceId, start, err, 0, "")
			return nil, err
		}
	}
	// Data source = SQL view definition; response columns = projection (optionally narrowed by req.ColumnIds).
	colIds, err := s.resolveDataSourceViewProjection(ctx, ds.TableId, ds.ColumnIds, req.ColumnIds)
	if err != nil {
		s.recordDataSourceQuery(ctx, tid, req.TableId, req.DataSourceId, start, err, 0, "")
		return nil, err
	}
	if len(req.ColumnIds) > 0 && len(colIds) == 0 {
		err := fmt.Errorf("no columns match data source projection")
		s.recordDataSourceQuery(ctx, tid, req.TableId, req.DataSourceId, start, err, 0, "")
		return nil, err
	}
	resp, err := s.executeQuery(ctx, querySpec{
		TableID:        ds.TableId,
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
	s.recordDataSourceQuery(ctx, tid, ds.TableId, req.DataSourceId, start, err, rowCount, ds.TableId)
	if err != nil {
		return nil, err
	}
	return &apiv1.QueryDataSourceResponse{
		Rows:          resp.Rows,
		NextPageToken: resp.NextPageToken,
		Count:         resp.Count,
	}, nil
}

func (s *Data) recordDataSourceQuery(ctx context.Context, tenantID, tableID, dataSourceName string, start time.Time, err error, rowCount int32, queryTableID string) {
	if tenantID == "" || tableID == "" || dataSourceName == "" {
		return
	}
	dsKey := tableID + "/" + dataSourceName
	duration := time.Since(start)
	s.B.DSMetrics.Record(ctx, tenantID, dsKey, duration, err)

	if s.B.Log == nil {
		return
	}
	attrs := []any{
		"tenant_id", tenantID,
		"table_id", tableID,
		"datasource_name", dataSourceName,
		"duration_ms", duration.Milliseconds(),
		"row_count", rowCount,
	}
	if queryTableID != "" && queryTableID != tableID {
		attrs = append(attrs, "query_table_id", queryTableID)
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		s.B.Log.Warn("datasource query failed", attrs...)
		return
	}
	if stats, statsErr := s.B.DSMetrics.Stats(ctx, tenantID, dsKey); statsErr == nil && stats.Count > 0 {
		attrs = append(attrs, "avg_duration_ms", stats.AvgDuration.Milliseconds(), "window_count", stats.Count)
	}
	if duration >= s.B.SlowQueryThreshold {
		s.B.Log.Warn("slow datasource query", attrs...)
	} else {
		s.B.Log.Info("datasource query", attrs...)
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

func (s *Data) loadDataSourceSpec(ctx context.Context, tableRef, dsName string) (*loadedDataSource, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	tableID, name, err := platform.New(s.B).ResolveDataSourceRef(ctx, tableRef, dsName)
	if err != nil {
		return nil, err
	}
	key := shared.CacheKeyDataSource(tid, tableID, name)
	if s.B.Cache != nil {
		var cached loadedDataSource
		if ok, _ := s.B.Cache.Get(ctx, key, &cached); ok {
			return &cached, nil
		}
	}

	meta := s.B.Tenants.MetaPool()
	var filter map[string]any
	var sortJSON []byte
	var colNames []string
	if err := meta.QueryRow(ctx, `
		SELECT filter, sort, column_names
		FROM lc_data_sources WHERE tenant_id = $1 AND table_id = $2 AND name = $3`,
		tid, tableID, name,
	).Scan(&filter, &sortJSON, &colNames); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("data source not found")
		}
		return nil, err
	}
	var sort []*apiv1.SortOrder
	if len(sortJSON) > 0 {
		_ = json.Unmarshal(sortJSON, &sort)
	}
	out := &loadedDataSource{TableId: tableID, Filter: filter, Sort: sort, ColumnIds: colNames}
	if s.B.Cache != nil {
		_ = s.B.Cache.Set(ctx, key, out, s.B.CacheTTL)
	}
	return out, nil
}

func (s *Data) executeQuery(ctx context.Context, spec querySpec) (resp *apiv1.QueryRowsResponse, execErr error) {
	start := time.Now()
	defer func() {
		if execErr != nil {
			s.logQueryExecution(spec.TableID, start, 0, 0, execErr)
		}
	}()

	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := spec.TableID
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}

	allCols, schemaName, tableName, err := catalog.New(s.B).LoadAllColumnMeta(ctx, tableID)
	if err != nil {
		return nil, err
	}
	physCols := filterPhysicalCols(allCols)
	if len(physCols) == 0 {
		return &apiv1.QueryRowsResponse{}, nil
	}

	selCols := physCols
	if spec.ColumnRestrict {
		want := map[string]struct{}{}
		for _, id := range spec.ColumnIds {
			want[id] = struct{}{}
		}
		var subset []shared.ColumnMeta
		for _, c := range physCols {
			if schema.ColumnRefInSet(c, want) {
				subset = append(subset, c)
			}
		}
		selCols = subset
	} else if len(spec.ColumnIds) > 0 {
		want := map[string]struct{}{}
		for _, id := range spec.ColumnIds {
			want[id] = struct{}{}
		}
		var subset []shared.ColumnMeta
		for _, c := range physCols {
			if schema.ColumnRefInSet(c, want) {
				subset = append(subset, c)
			}
		}
		if len(subset) > 0 {
			selCols = subset
		}
	}
	var colAllow map[string]struct{}
	if spec.ColumnRestrict {
		colAllow = columnAllowSet(spec.ColumnIds)
	}

	pageSize := normalizePageSize(spec.PageSize, s.B.MaxRow)

	rollupPlans, err := s.buildRollupPlans(ctx, tableID, allCols)
	if err != nil {
		return nil, err
	}

	idPgType, err := schema.New(s.B).LoadTableIDPgType(ctx, schemaName, tableName)
	if err != nil {
		return nil, err
	}

	const baseAlias = "_b"
	columnSQL := baseAlias + ".id::text"
	qCols := make([]query.ColumnMeta, len(selCols))
	for i, c := range selCols {
		columnSQL += ", " + baseAlias + "." + pgx.Identifier{c.Name}.Sanitize()
		qCols[i] = query.ColumnMeta{ID: c.Id, Name: c.Name, PgType: c.PgType}
	}
	attrMap := query.AttrMapFromColumns(baseAlias, qCols)
	for _, c := range physCols {
		attrMap[c.Id] = baseAlias + "." + pgx.Identifier{c.Name}.Sanitize()
		attrMap[c.Name] = baseAlias + "." + pgx.Identifier{c.Name}.Sanitize()
	}

	args := []any{}
	whereParts := []string{}

	if spec.PageToken != "" {
		whereParts = append(whereParts, schema.PageTokenIDCompare(baseAlias, idPgType, len(args)+1))
		args = append(args, spec.PageToken)
	}

	argAcc := &argAccumulator{args: &args}
	lookupSpecs, err := s.buildLookupJoinSpecs(ctx, tableID, argAcc)
	if err != nil {
		return nil, err
	}

	type rollupComputed struct {
		ColumnName string
		SQL        string
	}
	var rollupComputedSpecs []rollupComputed
	rollupSQLByName := map[string]string{}
	for _, plan := range rollupPlans {
		rSQL, rArgs, err := s.buildRollupSQL(plan, baseAlias, len(args)+1)
		if err != nil {
			return nil, err
		}
		rollupComputedSpecs = append(rollupComputedSpecs, rollupComputed{ColumnName: plan.ColumnName, SQL: rSQL})
		rollupSQLByName[plan.ColumnName] = rSQL
		args = append(args, rArgs...)
	}

	formulaSteps, err := s.buildFormulaSteps(allCols, baseAlias, lookupSpecs, rollupSQLByName)
	if err != nil {
		return nil, err
	}
	extendAttrMapVirtual(attrMap, allCols, lookupSpecs, rollupSQLByName, formulaSteps)

	fromSQL := fmt.Sprintf(`%s.%s AS %s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		pgx.Identifier{baseAlias}.Sanitize(),
	)
	for _, step := range formulaSteps {
		fromSQL += step.LateralJoinSQL()
	}
	hostRelJoined := map[string]struct{}{}
	for _, lk := range lookupSpecs {
		relKey := hostRelJoinKey(lk.TargetSchema, lk.TargetTable, lk.BaseFKPgCol)
		if _, done := hostRelJoined[relKey]; !done {
			onSQL := fmt.Sprintf(`%s.%s = %s.id`,
				pgx.Identifier{baseAlias}.Sanitize(),
				pgx.Identifier{lk.BaseFKPgCol}.Sanitize(),
				pgx.Identifier{lk.Alias}.Sanitize(),
			)
			if len(lk.Filter) > 0 && len(lk.TargetCols) > 0 {
				filterSQL, filterArgs, err := linkedTableFilterSQL(map[string]any{"filter": lk.Filter}, lk.Alias, lk.TargetCols, len(args)+1)
				if err != nil {
					return nil, err
				}
				if filterSQL != "" {
					onSQL += " AND (" + filterSQL + ")"
					args = append(args, filterArgs...)
				}
			}
			fromSQL += fmt.Sprintf(` LEFT JOIN %s.%s AS %s ON %s`,
				pgx.Identifier{lk.TargetSchema}.Sanitize(),
				pgx.Identifier{lk.TargetTable}.Sanitize(),
				pgx.Identifier{lk.Alias}.Sanitize(),
				onSQL,
			)
			hostRelJoined[relKey] = struct{}{}
		}
		fromSQL += lk.ExtraFromSQL
	}

	filterWhere, err := dsl.Parse(spec.Filter)
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}
	if filterWhere.Type != "" {
		wSQL, wArgs, err := query.BuildWhere(filterWhere, attrMap, len(args)+1)
		if err != nil {
			return nil, err
		}
		if wSQL != "" {
			whereParts = append(whereParts, wSQL)
			args = append(args, wArgs...)
		}
	}

	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = " WHERE " + strings.Join(whereParts, " AND ")
	}

	for _, step := range formulaSteps {
		columnSQL += ", " + step.SelectRef() + " AS " + pgx.Identifier{"f_" + step.Name}.Sanitize()
	}
	for _, r := range rollupComputedSpecs {
		columnSQL += ", (" + r.SQL + ") AS " + pgx.Identifier{"r_" + r.ColumnName}.Sanitize()
	}
	for _, lk := range lookupSpecs {
		columnSQL += ", " + lk.SelectExpr + " AS " + pgx.Identifier{"lkval_" + lk.LookupColumnName}.Sanitize()
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

	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s%s`, fromSQL, whereSQL)
	var total int32
	s.logSQL("count", tableID, countSQL, args)
	if err := data.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	limitArg := len(args) + 1
	queryArgs := append(append([]any{}, args...), pageSize+1)

	querySQL := fmt.Sprintf(`SELECT %s FROM %s%s%s LIMIT $%d`,
		columnSQL, fromSQL, whereSQL, orderClause, limitArg,
	)
	s.logSQL("select", tableID, querySQL, queryArgs)
	rows, err := data.Query(ctx, querySQL, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out apiv1.QueryRowsResponse
	var lastID string
	for rows.Next() {
		nScan := 1 + len(selCols) + len(formulaSteps) + len(rollupComputedSpecs) + len(lookupSpecs)
		scanTargets := make([]any, nScan)
		var id string
		scanTargets[0] = &id
		values := make([]any, len(selCols))
		for i := range values {
			values[i] = new(any)
			scanTargets[i+1] = values[i]
		}
		off := 1 + len(selCols)
		formulaVals := make([]*any, len(formulaSteps))
		for i := range formulaSteps {
			formulaVals[i] = new(any)
			scanTargets[off+i] = formulaVals[i]
		}
		off += len(formulaSteps)
		rollupVals := make([]*any, len(rollupComputedSpecs))
		for i := range rollupComputedSpecs {
			rollupVals[i] = new(any)
			scanTargets[off+i] = rollupVals[i]
		}
		off += len(rollupComputedSpecs)
		lkVals := make([]*any, len(lookupSpecs))
		for i := range lookupSpecs {
			lkVals[i] = new(any)
			scanTargets[off+i] = lkVals[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, err
		}
		lastID = id

		row := &apiv1.Row{Id: id, Cells: make(map[string]*apiv1.Value)}
		for i, c := range selCols {
			vPtr := values[i].(*any)
			if *vPtr != nil {
				row.Cells[c.Name] = shared.DBCellValue(*vPtr, c.PgType)
			}
		}
		for i, lk := range lookupSpecs {
			if lkVals[i] != nil && *lkVals[i] != nil {
				row.Cells[lk.LookupColumnName] = shared.DBCellValue(*lkVals[i], lk.TargetValuePgType)
			} else {
				row.Cells[lk.LookupColumnName] = &apiv1.Value{JsonValue: json.RawMessage("null")}
			}
		}
		for i, step := range formulaSteps {
			if formulaVals[i] != nil && *formulaVals[i] != nil {
				row.Cells[step.Name] = shared.DBCellValue(*formulaVals[i], "")
			} else {
				row.Cells[step.Name] = &apiv1.Value{JsonValue: json.RawMessage("null")}
			}
		}
		for i, r := range rollupComputedSpecs {
			if rollupVals[i] != nil && *rollupVals[i] != nil {
				row.Cells[r.ColumnName] = shared.DBCellValue(*rollupVals[i], "")
			}
		}

		if colAllow != nil {
			filtered := make(map[string]*apiv1.Value, len(row.Cells))
			for k, v := range row.Cells {
				if columnAllowed(k, colAllow) {
					filtered[k] = v
				}
			}
			row.Cells = filtered
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

func (s *Data) logSQL(op, tableID, sql string, args []any) {
	if s.B.Log == nil || !s.B.LogSQL {
		return
	}
	s.B.Log.Info("sql query",
		"op", op,
		"table_id", tableID,
		"sql", sql,
		"args", logger.FormatSQLArgs(args),
	)
}

func (s *Data) logQueryExecution(tableID string, start time.Time, rowCount int, total int32, err error) {
	if s.B.Log == nil {
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
		s.B.Log.Warn("query failed", attrs...)
		return
	}
	if s.B.LogSQL {
		s.B.Log.Info("query done", attrs...)
		return
	}
	if duration >= s.B.SlowQueryThreshold {
		s.B.Log.Warn("slow query", attrs...)
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

type rollupPlan struct {
	ColumnName   string
	Aggregate    string
	TargetPgCol  string
	LinkPgCol    string
	TargetSchema string
	TargetTable  string
	Filter       map[string]any
	TargetCols   []shared.ColumnMeta
}

func (s *Data) buildRollupSQL(plan rollupPlan, baseAlias string, argStart int) (string, []any, error) {
	extraWhere := ""
	var args []any
	if len(plan.Filter) > 0 && len(plan.TargetCols) > 0 {
		wSQL, wArgs, err := linkedTableFilterSQL(map[string]any{"filter": plan.Filter}, "_r", plan.TargetCols, argStart)
		if err != nil {
			return "", nil, err
		}
		extraWhere = wSQL
		args = wArgs
	}
	sql := shared.RollupAggregateSQL(plan.Aggregate, plan.TargetPgCol, plan.LinkPgCol, plan.TargetSchema, plan.TargetTable, baseAlias, extraWhere)
	return sql, args, nil
}

func (s *Data) buildFormulaSteps(
	allCols []shared.FullColumnMeta,
	baseAlias string,
	lookupSpecs []lookupJoinSpec,
	rollupSQLByName map[string]string,
) ([]formulacompile.Step, error) {
	defs := shared.FormulaDefs(allCols)
	if len(defs) == 0 {
		return nil, nil
	}
	baseRefs := s.formulaNameToPg(allCols, lookupSpecs, rollupSQLByName)
	return formulacompile.BuildSteps(baseAlias, baseRefs, defs)
}

func (s *Data) formulaNameToPg(
	allCols []shared.FullColumnMeta,
	lookupSpecs []lookupJoinSpec,
	rollupSQLByName map[string]string,
) map[string]string {
	nameToPg := map[string]string{}
	for _, c := range allCols {
		if shared.FormulaRefAllowed(c.Kind) {
			nameToPg[c.Name] = c.Name
		}
	}
	for _, lk := range lookupSpecs {
		nameToPg[lk.LookupColumnName] = lk.SelectExpr
	}
	for name, sql := range rollupSQLByName {
		nameToPg[name] = sql
	}
	return nameToPg
}

func (s *Data) buildRollupPlans(ctx context.Context, tableID string, allCols []shared.FullColumnMeta) ([]rollupPlan, error) {
	var rollups []rollupPlan
	for _, c := range allCols {
		switch c.Kind {
		case "rollup":
			relID := shared.CfgString(c.Config, "relation_column_id")
			fieldID := shared.CfgString(c.Config, "target_column_id")
			agg := shared.CfgString(c.Config, "aggregate")
			if relID == "" {
				continue
			}
			rels, err := schema.New(s.B).LoadRelationshipColumns(ctx, tableID, []string{relID})
			if err != nil || len(rels) == 0 {
				continue
			}
			rel := rels[0]
			tgtCols, tgtSchema, tgtTable, err := catalog.New(s.B).LoadColumns(ctx, rel.TargetTableId)
			if err != nil {
				continue
			}
			var linkPg, targetPg string
			tid, _ := s.B.TenantID(ctx)
			if rel.LinkColumnId != "" {
				linkPg, _ = schema.New(s.B).ColumnPgColumnByRef(ctx, tid, rel.TargetTableId, rel.LinkColumnId)
			}
			for _, tc := range tgtCols {
				if schema.ColumnRefMatches(tc, fieldID) {
					targetPg = tc.Name
					break
				}
			}
			if linkPg == "" {
				continue
			}
			var filter map[string]any
			if raw, ok := c.Config["filter"].(map[string]any); ok && len(raw) > 0 {
				filter = raw
			}
			rollups = append(rollups, rollupPlan{
				ColumnName:   c.Name,
				Aggregate:    agg,
				TargetPgCol:  targetPg,
				LinkPgCol:    linkPg,
				TargetSchema: tgtSchema,
				TargetTable:  tgtTable,
				Filter:       filter,
				TargetCols:   tgtCols,
			})
		}
	}
	return rollups, nil
}

func filterPhysicalCols(all []shared.FullColumnMeta) []shared.ColumnMeta {
	var out []shared.ColumnMeta
	for _, c := range all {
		if !c.IsVirtual {
			out = append(out, shared.ColumnMeta{Id: c.Id, TableId: c.TableId, Name: c.Name, TypeId: c.TypeId, PgType: c.PgType, IsNullable: c.IsNullable, Position: c.Position})
		}
	}
	return out
}

func (s *Data) applyExpansions(ctx context.Context, tableID string, row *apiv1.Row, expandIDs, expandPaths []string) error {
	if len(expandIDs) > 0 {
		relCols, err := schema.New(s.B).LoadRelationshipColumns(ctx, tableID, expandIDs)
		if err != nil {
			return err
		}
		for _, rel := range relCols {
			related, err := s.fetchRelatedRows(ctx, tableID, rel, row.Id, row.Cells, fetchRelatedOpts{})
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
