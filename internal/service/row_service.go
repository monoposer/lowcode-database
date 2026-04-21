package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/webhook"
)

const (
	maxExpandPathDepth = 5
	maxExpandManyRows  = 100
)

// fetchRelatedOpts controls optional projection when loading related rows.
type fetchRelatedOpts struct {
	SelectColumnIDs []string // ids of physical columns on the target table; empty = all physical columns
}

// -------- Row / Cell --------

func (s *LowcodeService) CreateRow(ctx context.Context, req *apiv1.CreateRowRequest) (*apiv1.CreateRowResponse, error) {
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}

	cols, schemaName, tableName, err := s.loadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}

	if len(req.Cells) == 0 {
		return nil, fmt.Errorf("cells is empty")
	}

	var pgCols []string
	var args []any
	for _, c := range cols {
		val, ok := req.Cells[c.Id]
		if !ok {
			continue
		}
		pgCols = append(pgCols, c.PgColumn)
		args = append(args, valueToAnyForColumn(val, c.PgType))
	}

	if len(pgCols) == 0 {
		return nil, fmt.Errorf("no valid cells for known columns")
	}

	colsSQL := strings.Join(pgCols, ", ")
	params := make([]string, len(pgCols))
	for i := range params {
		params[i] = fmt.Sprintf("$%d", i+1)
	}
	paramSQL := strings.Join(params, ", ")

	insert := fmt.Sprintf(`INSERT INTO %s.%s (%s) VALUES (%s) RETURNING id`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		colsSQL,
		paramSQL,
	)

	var rowID string
	if err := data.QueryRow(ctx, insert, args...).Scan(&rowID); err != nil {
		return nil, err
	}

	resp := &apiv1.CreateRowResponse{
		Row: &apiv1.Row{
			Id:    rowID,
			Cells: req.Cells,
		},
	}
	if s.Hooks != nil {
		s.Hooks.Emit(ctx, webhook.RecordsAfterInsert, tableID, map[string]any{
			"row": RowToMap(resp.Row),
		})
	}
	return resp, nil
}

// 这里为了简单，只实现按 id 精确匹配的 UpdateRow，BulkUpsertRows 里会复用。
func (s *LowcodeService) UpdateRow(ctx context.Context, req *apiv1.UpdateRowRequest) (*apiv1.UpdateRowResponse, error) {
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" || req.RowId == "" {
		return nil, fmt.Errorf("table_id and row_id are required")
	}

	cols, schemaName, tableName, err := s.loadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if len(req.Cells) == 0 {
		return nil, fmt.Errorf("cells is empty")
	}

	var setParts []string
	var args []any
	argIdx := 1
	for _, c := range cols {
		val, ok := req.Cells[c.Id]
		if !ok {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = $%d", c.PgColumn, argIdx))
		args = append(args, valueToAnyForColumn(val, c.PgType))
		argIdx++
	}
	if len(setParts) == 0 {
		return nil, fmt.Errorf("no valid cells for known columns")
	}

	args = append(args, req.RowId)
	update := fmt.Sprintf(`UPDATE %s.%s SET %s WHERE id = $%d`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		strings.Join(setParts, ", "),
		argIdx,
	)
	if _, err := data.Exec(ctx, update, args...); err != nil {
		return nil, err
	}

	resp := &apiv1.UpdateRowResponse{
		Row: &apiv1.Row{
			Id:    req.RowId,
			Cells: req.Cells,
		},
	}
	if s.Hooks != nil {
		s.Hooks.Emit(ctx, webhook.RecordsAfterUpdate, tableID, map[string]any{
			"row": RowToMap(resp.Row),
		})
	}
	return resp, nil
}

func (s *LowcodeService) DeleteRow(ctx context.Context, req *apiv1.DeleteRowRequest) (*apiv1.DeleteRowResponse, error) {
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" || req.RowId == "" {
		return nil, fmt.Errorf("table_id and row_id are required")
	}

	_, schemaName, tableName, err := s.loadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}

	del := fmt.Sprintf(`DELETE FROM %s.%s WHERE id = $1`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize())
	if _, err := data.Exec(ctx, del, req.RowId); err != nil {
		return nil, err
	}
	if s.Hooks != nil {
		s.Hooks.Emit(ctx, webhook.RecordsAfterDelete, tableID, map[string]any{
			"rowId": req.RowId,
		})
	}
	return &apiv1.DeleteRowResponse{}, nil
}

type lookupJoinSpec struct {
	LookupColumnID string
	Alias          string
	TargetSchema   string
	TargetTable    string
	TargetPgCol    string
	BaseFKPgCol    string
}

func (s *LowcodeService) buildLookupJoinSpecs(ctx context.Context, tableID string) ([]lookupJoinSpec, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	resolvedName, err := s.resolveTableName(ctx, tableID)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	const q = `
		SELECT c.id, c.config
		FROM lc_columns c
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.type_id = 'lookup'
	`
	rows, err := meta.Query(ctx, q, resolvedName, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var specs []lookupJoinSpec
	for rows.Next() {
		var id string
		var cfg map[string]any
		if err := rows.Scan(&id, &cfg); err != nil {
			return nil, err
		}
		relID := cfgString(cfg, "relation_column_id")
		fieldID := cfgString(cfg, "target_column_id")
		if relID == "" || fieldID == "" {
			continue
		}
		rels, err := s.loadRelationshipColumns(ctx, tableID, []string{relID})
		if err != nil || len(rels) == 0 {
			continue
		}
		rel := rels[0]
		if rel.Cardinality != "one" || rel.TargetColumnId == "" {
			continue
		}
		var baseFKPg string
		if err := meta.QueryRow(ctx, `SELECT pg_column FROM lc_columns WHERE id = $1 AND tenant_id = $2`, rel.TargetColumnId, tid).Scan(&baseFKPg); err != nil {
			continue
		}
		targetResolved, err := s.resolveTableName(ctx, rel.TargetTableId)
		if err != nil {
			continue
		}
		var tgtSchema, tgtTable, tgtPg string
		if err := meta.QueryRow(ctx, `
			SELECT t.schema_name, t.table_name, c.pg_column
			FROM lc_columns c
			JOIN lc_tables t ON c.table_id = t.name AND c.tenant_id = t.tenant_id
			WHERE c.id = $1 AND c.tenant_id = $2 AND c.table_id = $3`,
			fieldID, tid, targetResolved,
		).Scan(&tgtSchema, &tgtTable, &tgtPg); err != nil {
			continue
		}
		short := strings.ReplaceAll(id, "-", "")
		if len(short) > 8 {
			short = short[:8]
		}
		alias := "lk_" + short
		specs = append(specs, lookupJoinSpec{
			LookupColumnID: id,
			Alias:          alias,
			TargetSchema:   tgtSchema,
			TargetTable:    tgtTable,
			TargetPgCol:    tgtPg,
			BaseFKPgCol:    baseFKPg,
		})
	}
	return specs, rows.Err()
}

// ListRows delegates to QueryRows for filter/sort/pagination support.
func (s *LowcodeService) ListRows(ctx context.Context, req *apiv1.ListRowsRequest) (*apiv1.ListRowsResponse, error) {
	qresp, err := s.QueryRows(ctx, &apiv1.QueryRowsRequest{
		TableId:         req.TableId,
		PageSize:        req.PageSize,
		PageToken:       req.PageToken,
		ExpandColumnIds: req.ExpandColumnIds,
		ExpandPaths:     req.ExpandPaths,
	})
	if err != nil {
		return nil, err
	}
	return &apiv1.ListRowsResponse{
		Rows:          qresp.Rows,
		NextPageToken: qresp.NextPageToken,
	}, nil
}

func relationshipExpandValue(rel relationshipColumn, related []map[string]any) *apiv1.Value {
	if rel.Cardinality == "one" {
		if len(related) == 0 {
			return &apiv1.Value{JsonValue: json.RawMessage("null")}
		}
		return apiv1.JsonValue(related[0])
	}
	if related == nil {
		related = []map[string]any{}
	}
	rowsAny := make([]any, len(related))
	for i := range related {
		rowsAny[i] = related[i]
	}
	return apiv1.JsonValue(map[string]any{"rows": rowsAny})
}

func (s *LowcodeService) expandPathResult(ctx context.Context, tableID, rowID string, rowCells map[string]*apiv1.Value, parts []string, depth int) (any, error) {
	if depth > maxExpandPathDepth {
		return nil, fmt.Errorf("max depth %d exceeded", maxExpandPathDepth)
	}
	if len(parts) < 2 {
		return nil, fmt.Errorf("path segment too short")
	}
	relID := parts[0]
	rels, err := s.loadRelationshipColumns(ctx, tableID, []string{relID})
	if err != nil {
		return nil, err
	}
	if len(rels) == 0 {
		return nil, fmt.Errorf("unknown relationship column %q on table", relID)
	}
	rel := rels[0]
	rest := parts[1:]
	var opts fetchRelatedOpts
	if len(rest) == 1 {
		opts.SelectColumnIDs = []string{rest[0]}
	}
	related, err := s.fetchRelatedRows(ctx, rel, rowID, rowCells, opts)
	if err != nil {
		return nil, err
	}
	if rel.Cardinality == "one" {
		if len(related) == 0 {
			return map[string]any{"id": nil, "cells": map[string]any{}}, nil
		}
		r0 := related[0]
		if len(rest) == 1 {
			return r0, nil
		}
		cellsMap, _ := r0["cells"].(map[string]any)
		childID, _ := r0["id"].(string)
		return s.expandPathResult(ctx, rel.TargetTableId, childID, cellsAnyToValues(cellsMap), rest, depth+1)
	}
	out := []any{}
	for i, r := range related {
		if i >= maxExpandManyRows {
			break
		}
		cellsMap, _ := r["cells"].(map[string]any)
		childID, _ := r["id"].(string)
		if len(rest) == 1 {
			out = append(out, r)
			continue
		}
		sub, err := s.expandPathResult(ctx, rel.TargetTableId, childID, cellsAnyToValues(cellsMap), rest, depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, nil
}

func cellsAnyToValues(m map[string]any) map[string]*apiv1.Value {
	if m == nil {
		return map[string]*apiv1.Value{}
	}
	out := make(map[string]*apiv1.Value, len(m))
	for k, v := range m {
		out[k] = anyToValue(v)
	}
	return out
}

// fetchRelatedRows 根据 relationship 配置查询关联行，每项为 { "id", "cells" }（cells 为列 id -> 原生 JSON 值）。
func (s *LowcodeService) fetchRelatedRows(ctx context.Context, rel relationshipColumn, currentRowID string, currentRowCells map[string]*apiv1.Value, opts fetchRelatedOpts) ([]map[string]any, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	targetCols, targetSchema, targetTable, err := s.loadColumns(ctx, rel.TargetTableId)
	if err != nil {
		return nil, err
	}
	if len(targetCols) == 0 {
		return nil, nil
	}

	selCols := targetCols
	if len(opts.SelectColumnIDs) > 0 {
		want := map[string]struct{}{}
		for _, id := range opts.SelectColumnIDs {
			want[id] = struct{}{}
		}
		var subset []columnMeta
		for _, c := range targetCols {
			if _, ok := want[c.Id]; ok {
				subset = append(subset, c)
			}
		}
		if len(subset) > 0 {
			selCols = subset
		}
	}

	var query string
	var args []any

	if rel.Cardinality == "many" {
		if rel.LinkColumnId == "" {
			return nil, nil
		}
		var linkPgCol string
		meta := s.tenants.MetaPool()
		if err := meta.QueryRow(ctx, `SELECT pg_column FROM lc_columns WHERE id = $1 AND tenant_id = $2`, rel.LinkColumnId, tid).Scan(&linkPgCol); err != nil {
			if err == pgx.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		columnSQL := "id"
		for _, c := range selCols {
			columnSQL += ", " + c.PgColumn
		}
		query = fmt.Sprintf(`SELECT %s FROM %s.%s WHERE %s = $1 ORDER BY id`,
			columnSQL,
			pgx.Identifier{targetSchema}.Sanitize(),
			pgx.Identifier{targetTable}.Sanitize(),
			pgx.Identifier{linkPgCol}.Sanitize(),
		)
		args = []any{currentRowID}
	} else {
		relatedID := ""
		if v, ok := currentRowCells[rel.TargetColumnId]; ok && v != nil && v.StringValue != nil {
			relatedID = *v.StringValue
		}
		if relatedID == "" {
			return nil, nil
		}
		columnSQL := "id"
		for _, c := range selCols {
			columnSQL += ", " + c.PgColumn
		}
		query = fmt.Sprintf(`SELECT %s FROM %s.%s WHERE id = $1`,
			columnSQL,
			pgx.Identifier{targetSchema}.Sanitize(),
			pgx.Identifier{targetTable}.Sanitize(),
		)
		args = []any{relatedID}
	}

	rows, err := data.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []map[string]any
	for rows.Next() {
		scanTargets := make([]any, 1+len(selCols))
		var rowID string
		scanTargets[0] = &rowID
		values := make([]any, len(selCols))
		for i := range values {
			values[i] = new(any)
			scanTargets[i+1] = values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, err
		}
		cellsMap := make(map[string]any, len(selCols))
		for i, c := range selCols {
			vPtr := values[i].(*any)
			if *vPtr != nil {
				cellsMap[c.Id] = *vPtr
			}
		}
		list = append(list, map[string]any{"id": rowID, "cells": cellsMap})
	}
	return list, rows.Err()
}
