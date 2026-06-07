package data

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/shared"
	"github.com/solat/lowcode-database/internal/sink"
)

// -------- Bulk --------

func (s *Data) BulkUpsertRows(ctx context.Context, req *apiv1.BulkUpsertRowsRequest) (*apiv1.BulkUpsertRowsResponse, error) {
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}
	cols, schemaName, tableName, err := catalog.New(s.B).LoadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns for table")
	}

	tx, err := data.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var resp apiv1.BulkUpsertRowsResponse

	for _, item := range req.Items {
		if item == nil {
			continue
		}
		if item.RowId == "" {
			id, err := s.insertRowTx(ctx, tx, cols, schemaName, tableName, shared.NormalizeInputCells(item.Cells, cols))
			if err != nil {
				return nil, err
			}
			if id == "" {
				continue
			}
			resp.Rows = append(resp.Rows, &apiv1.Row{Id: id, Cells: shared.NormalizeInputCells(item.Cells, cols)})
		} else {
			if err := s.updateRowTx(ctx, tx, cols, schemaName, tableName, item.RowId, item.Cells); err != nil {
				return nil, err
			}
			resp.Rows = append(resp.Rows, &apiv1.Row{Id: item.RowId, Cells: shared.NormalizeInputCells(item.Cells, cols)})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.B.Hooks != nil && len(resp.Rows) > 0 {
		rows := make([]any, 0, len(resp.Rows))
		for _, r := range resp.Rows {
			rows = append(rows, shared.RowToMap(r))
		}
		s.B.Hooks.Emit(ctx, sink.RecordsAfterBulkUpsert, tableID, map[string]any{
			"rows": rows,
		})
	}
	return &resp, nil
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

func (s *Data) BulkDeleteRows(ctx context.Context, req *apiv1.BulkDeleteRowsRequest) (*apiv1.BulkDeleteRowsResponse, error) {
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}
	_, schemaName, tableName, err := catalog.New(s.B).LoadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if len(req.RowIds) == 0 {
		return &apiv1.BulkDeleteRowsResponse{}, nil
	}

	tx, err := data.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	del := fmt.Sprintf(`DELETE FROM %s.%s WHERE id = ANY($1)`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
	)
	if _, err := tx.Exec(ctx, del, req.RowIds); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.B.Hooks != nil && len(req.RowIds) > 0 {
		ids := make([]any, len(req.RowIds))
		for i, id := range req.RowIds {
			ids[i] = id
		}
		s.B.Hooks.Emit(ctx, sink.RecordsAfterBulkDelete, tableID, map[string]any{
			"rowIds": ids,
		})
	}
	return &apiv1.BulkDeleteRowsResponse{}, nil
}
