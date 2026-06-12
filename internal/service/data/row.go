package data

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/apiv1/row"
	"github.com/solat/lowcode-database/internal/event"
	"github.com/solat/lowcode-database/internal/service/shared"
	"strings"
)

func (s *Data) CreateRow(ctx context.Context, req *row.CreateRowRequest) (*row.CreateRowResponse, error) {
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}

	cols, schemaName, tableName, err := s.meta().LoadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}

	if len(req.Cells) == 0 {
		return nil, fmt.Errorf("cells is empty")
	}

	var pgCols []string
	var args []any
	for _, c := range cols {
		val, ok := shared.CellByRef(req.Cells, c)
		if !ok {
			continue
		}
		pgCols = append(pgCols, c.Name)
		args = append(args, shared.ValueToAnyForColumn(val, c.PgType))
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

	insert := fmt.Sprintf(`INSERT INTO %s.%s (%s) VALUES (%s) RETURNING id::text`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		colsSQL,
		paramSQL,
	)

	var rowID string
	if err := data.QueryRow(ctx, insert, args...).Scan(&rowID); err != nil {
		return nil, err
	}

	resp := &row.CreateRowResponse{
		Row: &row.Row{
			Id:    rowID,
			Cells: shared.NormalizeInputCells(req.Cells, cols),
		},
	}
	if s.B.Events != nil {
		s.B.EmitEvent(ctx, event.RecordsAfterInsert, tableID, map[string]any{
			"row": shared.RowToMap(resp.Row),
		})
	}
	return resp, nil
}

// UpdateRowByID updates a row matched by primary key; reused from BulkUpsertRows.
func (s *Data) UpdateRow(ctx context.Context, req *row.UpdateRowRequest) (*row.UpdateRowResponse, error) {
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" || req.RowId == "" {
		return nil, fmt.Errorf("table_id and row_id are required")
	}

	cols, schemaName, tableName, err := s.meta().LoadColumns(ctx, tableID)
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
		val, ok := shared.CellByRef(req.Cells, c)
		if !ok {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = $%d", pgx.Identifier{c.Name}.Sanitize(), argIdx))
		args = append(args, shared.ValueToAnyForColumn(val, c.PgType))
		argIdx++
	}
	if len(setParts) == 0 {
		return nil, fmt.Errorf("no valid cells for known columns")
	}

	setParts = shared.TouchUpdatedAtSQL(setParts)
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

	resp := &row.UpdateRowResponse{
		Row: &row.Row{
			Id:    req.RowId,
			Cells: shared.NormalizeInputCells(req.Cells, cols),
		},
	}
	if s.B.Events != nil {
		s.B.EmitEvent(ctx, event.RecordsAfterUpdate, tableID, map[string]any{
			"row": shared.RowToMap(resp.Row),
		})
	}
	return resp, nil
}

func (s *Data) DeleteRow(ctx context.Context, req *row.DeleteRowRequest) (*row.DeleteRowResponse, error) {
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" || req.RowId == "" {
		return nil, fmt.Errorf("table_id and row_id are required")
	}

	_, schemaName, tableName, err := s.meta().LoadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}

	del := fmt.Sprintf(`DELETE FROM %s.%s WHERE id = $1`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize())
	if _, err := data.Exec(ctx, del, req.RowId); err != nil {
		return nil, err
	}
	if s.B.Events != nil {
		s.B.EmitEvent(ctx, event.RecordsAfterDelete, tableID, map[string]any{
			"rowId": req.RowId,
		})
	}
	return &row.DeleteRowResponse{}, nil
}

// ListRows delegates to QueryRows for filter/sort/pagination support.
func (s *Data) ListRows(ctx context.Context, req *row.ListRowsRequest) (*row.ListRowsResponse, error) {
	qresp, err := s.QueryRows(ctx, &row.QueryRowsRequest{
		TableId:         req.TableId,
		PageSize:        req.PageSize,
		PageToken:       req.PageToken,
		ExpandColumnIds: req.ExpandColumnIds,
		ExpandPaths:     req.ExpandPaths,
	})
	if err != nil {
		return nil, err
	}
	return &row.ListRowsResponse{
		Rows:          qresp.Rows,
		NextPageToken: qresp.NextPageToken,
	}, nil
}

// insertRowTx inserts one row; cells keys are logical column names (legacy UUID accepted).
func (s *Data) insertRowTx(ctx context.Context, tx pgx.Tx, cols []shared.ColumnMeta, schemaName, tableName string, cells map[string]*apiv1.Value) (string, error) {
	cells = shared.NormalizeInputCells(cells, cols)
	var pgCols []string
	var args []any
	for _, c := range cols {
		val, ok := cells[c.Name]
		if !ok {
			continue
		}
		pgCols = append(pgCols, c.Name)
		args = append(args, shared.ValueToAnyForColumn(val, c.PgType))
	}
	if len(pgCols) == 0 {
		return "", nil
	}
	colsSQL := strings.Join(pgCols, ", ")
	params := make([]string, len(pgCols))
	for i := range params {
		params[i] = fmt.Sprintf("$%d", i+1)
	}
	paramSQL := strings.Join(params, ", ")
	insert := fmt.Sprintf(`INSERT INTO %s.%s (%s) VALUES (%s) RETURNING id::text`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		colsSQL, paramSQL)
	var id string
	if err := tx.QueryRow(ctx, insert, args...).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

const (
	maxExpandPathDepth = 5
	maxExpandManyRows  = 100
)

// fetchRelatedOpts controls optional projection when loading related rows.
type fetchRelatedOpts struct {
	SelectColumnIDs []string // ids of physical columns on the target table; empty = all physical columns
}

func relationshipExpandValue(rel shared.RelationshipColumn, related []map[string]any) *apiv1.Value {
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

func (s *Data) expandPathResult(ctx context.Context, tableID, rowID string, rowCells map[string]*apiv1.Value, parts []string, depth int) (any, error) {
	if depth > maxExpandPathDepth {
		return nil, fmt.Errorf("max depth %d exceeded", maxExpandPathDepth)
	}
	if len(parts) < 2 {
		return nil, fmt.Errorf("path segment too short")
	}
	relID := parts[0]
	rels, err := s.meta().LoadRelationshipColumns(ctx, tableID, []string{relID})
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
	related, err := s.fetchRelatedRows(ctx, tableID, rel, rowID, rowCells, opts)
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
		out[k] = shared.AnyToValue(v)
	}
	return out
}

// fetchRelatedRows loads related rows per relationship config; each item is { "id", "cells" } (column id -> native JSON).
func (s *Data) fetchRelatedRows(ctx context.Context, sourceTableID string, rel shared.RelationshipColumn, currentRowID string, currentRowCells map[string]*apiv1.Value, opts fetchRelatedOpts) ([]map[string]any, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	data, err := s.B.Tenants.DataReadPool(ctx)
	if err != nil {
		return nil, err
	}
	targetCols, targetSchema, targetTable, err := s.meta().LoadColumns(ctx, rel.TargetTableId)
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
		var subset []shared.ColumnMeta
		for _, c := range selCols {
			if _, ok := want[c.Name]; ok {
				subset = append(subset, c)
				continue
			}
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
		linkPgCol, err := s.meta().ColumnPgColumnByRef(ctx, tid, rel.TargetTableId, rel.LinkColumnId)
		if err != nil {
			return nil, nil
		}
		columnSQL := "id"
		for _, c := range selCols {
			columnSQL += ", " + pgx.Identifier{c.Name}.Sanitize()
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
		fkColName, err := s.meta().ColumnPgColumnByRef(ctx, tid, sourceTableID, rel.TargetColumnId)
		if err == nil {
			if v, ok := currentRowCells[fkColName]; ok && v != nil && v.StringValue != nil {
				relatedID = *v.StringValue
			}
		}
		if relatedID == "" {
			return nil, nil
		}
		columnSQL := "id"
		for _, c := range selCols {
			columnSQL += ", " + pgx.Identifier{c.Name}.Sanitize()
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
				cellsMap[c.Name] = shared.PGValueToNative(*vPtr, c.PgType)
			}
		}
		list = append(list, map[string]any{"id": rowID, "cells": cellsMap})
	}
	return list, rows.Err()
}
