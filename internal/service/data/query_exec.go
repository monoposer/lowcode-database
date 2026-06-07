package data

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/monoposer/lowcode-database/internal/apiv1"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
	"github.com/monoposer/lowcode-database/internal/dsl"
	formulacompile "github.com/monoposer/lowcode-database/internal/formula"
	"github.com/monoposer/lowcode-database/internal/logger"
	"github.com/monoposer/lowcode-database/internal/query"
	"github.com/monoposer/lowcode-database/internal/service/schema"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"strings"
	"time"
)

func (s *Data) executeQuery(ctx context.Context, spec querySpec) (resp *row.QueryRowsResponse, execErr error) {
	if err := s.rewriteVRLookupFilters(ctx, &spec); err != nil {
		return nil, err
	}
	return s.executeVRQuery(ctx, spec)
}

func (s *Data) scanQueryRows(ctx context.Context, plan *queryExecPlan, spec querySpec, data *pgxpool.Pool, pageSize int32, start time.Time) (*row.QueryRowsResponse, error) {
	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s%s`, plan.fromSQL, plan.whereSQL)
	var total int32
	s.logSQL("count", plan.tableID, countSQL, plan.args)
	if err := data.QueryRow(ctx, countSQL, plan.args...).Scan(&total); err != nil {
		return nil, err
	}

	limitArg := len(plan.args) + 1
	queryArgs := append(append([]any{}, plan.args...), pageSize+1)

	querySQL := fmt.Sprintf(`SELECT %s FROM %s%s%s LIMIT $%d`,
		plan.columnSQL, plan.fromSQL, plan.whereSQL, plan.orderClause, limitArg,
	)
	s.logSQL("select", plan.tableID, querySQL, queryArgs)
	rows, err := data.Query(ctx, querySQL, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out row.QueryRowsResponse
	var lastID string
	for rows.Next() {
		nScan := 1 + len(plan.selCols) + len(plan.rollupComputedSpecs) + len(plan.lookupSpecs)
		scanTargets := make([]any, nScan)
		var id string
		scanTargets[0] = &id
		values := make([]any, len(plan.selCols))
		for i := range values {
			values[i] = new(any)
			scanTargets[i+1] = values[i]
		}
		off := 1 + len(plan.selCols)
		rollupVals := make([]*any, len(plan.rollupComputedSpecs))
		for i := range plan.rollupComputedSpecs {
			rollupVals[i] = new(any)
			scanTargets[off+i] = rollupVals[i]
		}
		off += len(plan.rollupComputedSpecs)
		lkVals := make([]*any, len(plan.lookupSpecs))
		for i := range plan.lookupSpecs {
			lkVals[i] = new(any)
			scanTargets[off+i] = lkVals[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, err
		}
		lastID = id

		row := &row.Row{Id: id, Cells: make(map[string]*apiv1.Value)}
		for i, c := range plan.selCols {
			vPtr := values[i].(*any)
			if *vPtr != nil {
				row.Cells[c.Name] = shared.DBCellValue(*vPtr, c.PgType)
			}
		}
		for i, lk := range plan.lookupSpecs {
			if lkVals[i] != nil && *lkVals[i] != nil {
				row.Cells[lk.LookupColumnName] = shared.DBCellValue(*lkVals[i], lk.TargetValuePgType)
			} else {
				row.Cells[lk.LookupColumnName] = &apiv1.Value{JsonValue: json.RawMessage("null")}
			}
		}
		for i, r := range plan.rollupComputedSpecs {
			if rollupVals[i] != nil && *rollupVals[i] != nil {
				row.Cells[r.ColumnName] = shared.DBCellValue(*rollupVals[i], "")
			}
		}

		if plan.colAllow != nil {
			filtered := make(map[string]*apiv1.Value, len(row.Cells))
			for k, v := range row.Cells {
				if columnAllowed(k, plan.colAllow) {
					filtered[k] = v
				}
			}
			row.Cells = filtered
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
	s.logQueryExecution(plan.tableID, start, len(out.Rows), total, nil)
	return &out, nil
}

// querySpec holds merged query parameters from saved query / request.
type querySpec struct {
	TableID        string
	Filter         map[string]any
	Sort           []*apiv1.SortOrder
	ColumnIds      []string
	ColumnRestrict bool // when true, ColumnIds limits output (empty = id only)
	PageSize       int32
	PageToken      string
}

type loadedQuery struct {
	TableId   string
	Filter    map[string]any
	Sort      []*apiv1.SortOrder
	ColumnIds []string
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

func filterPhysicalCols(all []shared.FullColumnMeta) []shared.ColumnMeta {
	var out []shared.ColumnMeta
	for _, c := range all {
		if !c.IsVirtual {
			out = append(out, shared.ColumnMeta{Id: c.Id, TableId: c.TableId, Name: c.Name, TypeId: c.TypeId, PgType: c.PgType, IsNullable: c.IsNullable, Position: c.Position})
		}
	}
	return out
}

func (s *Data) recordSavedQuery(_ context.Context, tenantID, tableID, queryName string, start time.Time, err error, rowCount int32, queryTableID string) {
	if tenantID == "" || tableID == "" || queryName == "" {
		return
	}
	duration := time.Since(start)

	if s.B.Log == nil {
		return
	}
	attrs := []any{
		"tenant_id", tenantID,
		"table_id", tableID,
		"query_name", queryName,
		"db", "meta",
		"duration_ms", duration.Milliseconds(),
		"row_count", rowCount,
	}
	if queryTableID != "" && queryTableID != tableID {
		attrs = append(attrs, "query_table_id", queryTableID)
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		s.B.Log.Warn("query failed", attrs...)
		return
	}
	if duration >= s.B.SlowQueryThreshold {
		s.B.Log.Warn("slow query", attrs...)
	} else {
		s.B.Log.Info("query", attrs...)
	}
}

func (s *Data) logSQL(op, tableID, sql string, args []any) {
	if s.B.Log == nil || !s.B.LogSQL {
		return
	}
	s.B.Log.Info("sql query",
		"db", "data-shard",
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
		"db", "data-shard",
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

func (s *Data) buildQueryExecPlan(ctx context.Context, spec querySpec) (*queryExecPlan, error) {
	tableID := spec.TableID
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}

	allCols, schemaName, tableName, err := s.meta().LoadAllColumnMeta(ctx, tableID)
	if err != nil {
		return nil, err
	}
	physCols := filterPhysicalCols(allCols)
	if len(physCols) == 0 {
		return nil, nil
	}

	selCols := selectExecColumns(spec, physCols)

	var colAllow map[string]struct{}
	if spec.ColumnRestrict {
		colAllow = columnAllowSet(spec.ColumnIds)
	}

	pageSize := normalizePageSize(spec.PageSize, s.B.MaxRow)

	rollupPlans, err := s.buildRollupPlans(ctx, tableID, allCols)
	if err != nil {
		return nil, err
	}

	idPgType, err := s.meta().LoadTableIDPgType(ctx, schemaName, tableName)
	if err != nil {
		return nil, err
	}

	const baseAlias = "_b"
	qCols := make([]query.ColumnMeta, len(selCols))
	for i, c := range selCols {
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

	if err := s.buildFormulaSteps(allCols); err != nil {
		return nil, err
	}
	extendAttrMapVirtual(attrMap, allCols, lookupSpecs, rollupSQLByName)

	fromSQL, err := buildExecFromSQL(schemaName, tableName, baseAlias, lookupSpecs, &args)
	if err != nil {
		return nil, err
	}

	filterParts, err := appendExecFilterWhere(spec, physCols, lookupSpecs, attrMap, &args)
	if err != nil {
		return nil, err
	}
	whereParts = append(whereParts, filterParts...)

	columnSQL := buildExecColumnSQL(baseAlias, selCols, rollupComputedSpecs, lookupSpecs)
	orderClause := buildExecOrderClause(spec, attrMap)

	return &queryExecPlan{
		tableID:             tableID,
		selCols:             selCols,
		lookupSpecs:         lookupSpecs,
		rollupComputedSpecs: rollupComputedSpecs,
		fromSQL:             fromSQL,
		columnSQL:           columnSQL,
		whereSQL:            joinWhereParts(whereParts),
		orderClause:         orderClause,
		args:                args,
		colAllow:            colAllow,
		pageSize:            pageSize,
	}, nil
}

type rollupComputed struct {
	ColumnName string
	SQL        string
}

type queryExecPlan struct {
	tableID             string
	selCols             []shared.ColumnMeta
	lookupSpecs         []lookupJoinSpec
	rollupComputedSpecs []rollupComputed
	fromSQL             string
	columnSQL           string
	whereSQL            string
	orderClause         string
	args                []any
	colAllow            map[string]struct{}
	pageSize            int32
}

func selectExecColumns(spec querySpec, physCols []shared.ColumnMeta) []shared.ColumnMeta {
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
		return subset
	}
	if len(spec.ColumnIds) == 0 {
		return selCols
	}
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
		return subset
	}
	return selCols
}

func buildExecFromSQL(
	schemaName, tableName, baseAlias string,
	lookupSpecs []lookupJoinSpec,
	args *[]any,
) (string, error) {
	fromSQL := fmt.Sprintf(`%s.%s AS %s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		pgx.Identifier{baseAlias}.Sanitize(),
	)
	hostRelJoined := map[string]struct{}{}
	for _, lk := range lookupSpecs {
		if lk.Cardinality == "many" {
			continue
		}
		relKey := hostRelJoinKey(lk.TargetSchema, lk.TargetTable, lk.BaseFKPgCol)
		if _, done := hostRelJoined[relKey]; !done {
			onSQL := fmt.Sprintf(`%s.%s = %s.id`,
				pgx.Identifier{baseAlias}.Sanitize(),
				pgx.Identifier{lk.BaseFKPgCol}.Sanitize(),
				pgx.Identifier{lk.Alias}.Sanitize(),
			)
			if len(lk.Filter) > 0 && len(lk.TargetCols) > 0 {
				filterSQL, filterArgs, err := linkedTableFilterSQL(map[string]any{"filter": lk.Filter}, lk.Alias, lk.TargetCols, len(*args)+1)
				if err != nil {
					return "", err
				}
				if filterSQL != "" {
					onSQL += " AND (" + filterSQL + ")"
					*args = append(*args, filterArgs...)
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
	return fromSQL, nil
}

func appendExecFilterWhere(
	spec querySpec,
	physCols []shared.ColumnMeta,
	lookupSpecs []lookupJoinSpec,
	attrMap map[string]string,
	args *[]any,
) ([]string, error) {
	filterWhere, err := dsl.ParseCached(spec.Filter)
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}
	if filterWhere.Type == "" {
		return nil, nil
	}
	attrPgTypes := map[string]string{}
	for _, c := range physCols {
		if c.PgType == "" {
			continue
		}
		attrPgTypes[c.Id] = c.PgType
		attrPgTypes[c.Name] = c.PgType
	}
	for _, lk := range lookupSpecs {
		if lk.TargetValuePgType != "" {
			attrPgTypes[lk.LookupColumnName] = lk.TargetValuePgType
		}
	}
	wSQL, wArgs, err := query.BuildWhereWithTypesCached(filterWhere, attrMap, attrPgTypes, len(*args)+1)
	if err != nil {
		return nil, err
	}
	if wSQL == "" {
		return nil, nil
	}
	*args = append(*args, wArgs...)
	return []string{wSQL}, nil
}

func buildExecColumnSQL(
	baseAlias string,
	selCols []shared.ColumnMeta,
	rollupComputedSpecs []rollupComputed,
	lookupSpecs []lookupJoinSpec,
) string {
	columnSQL := baseAlias + ".id::text"
	for _, c := range selCols {
		columnSQL += ", " + baseAlias + "." + pgx.Identifier{c.Name}.Sanitize()
	}
	for _, r := range rollupComputedSpecs {
		columnSQL += ", (" + r.SQL + ") AS " + pgx.Identifier{"r_" + r.ColumnName}.Sanitize()
	}
	for _, lk := range lookupSpecs {
		columnSQL += ", " + lk.SelectExpr + " AS " + pgx.Identifier{"lkval_" + lk.LookupColumnName}.Sanitize()
	}
	return columnSQL
}

func buildExecOrderClause(spec querySpec, attrMap map[string]string) string {
	var orders []query.OrderSpec
	for _, o := range spec.Sort {
		orders = append(orders, query.OrderSpec{Attribute: o.Attribute, SortOrder: o.SortOrder})
	}
	orderSQL := query.BuildOrderBy(orders, attrMap, "id")
	if orderSQL == "" {
		return ""
	}
	return " ORDER BY " + orderSQL
}

func joinWhereParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(parts, " AND ")
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
			rels, err := s.meta().LoadRelationshipColumns(ctx, tableID, []string{relID})
			if err != nil || len(rels) == 0 {
				continue
			}
			rel := rels[0]
			tgtCols, tgtSchema, tgtTable, err := s.meta().LoadColumns(ctx, rel.TargetTableId)
			if err != nil {
				continue
			}
			var linkPg, targetPg string
			tid, _ := s.B.TenantID(ctx)
			if rel.LinkColumnId != "" {
				linkPg, _ = s.meta().ColumnPgColumnByRef(ctx, tid, rel.TargetTableId, rel.LinkColumnId)
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

func (s *Data) buildFormulaSteps(allCols []shared.FullColumnMeta) error {
	return formulacompile.BuildSteps("", nil, shared.FormulaDefs(allCols))
}
