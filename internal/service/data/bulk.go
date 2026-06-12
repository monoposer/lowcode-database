package data

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"strings"
)

func (s *Data) BulkUpsertRows(ctx context.Context, req *row.BulkUpsertRowsRequest) (*row.BulkUpsertRowsResponse, error) {
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
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns for table")
	}

	tx, err := data.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var resp row.BulkUpsertRowsResponse

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
			resp.Rows = append(resp.Rows, &row.Row{Id: id, Cells: shared.NormalizeInputCells(item.Cells, cols)})
		} else {
			if err := s.updateRowTx(ctx, tx, cols, schemaName, tableName, item.RowId, item.Cells); err != nil {
				return nil, err
			}
			resp.Rows = append(resp.Rows, &row.Row{Id: item.RowId, Cells: shared.NormalizeInputCells(item.Cells, cols)})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.B.Events != nil && len(resp.Rows) > 0 {
		rows := make([]any, 0, len(resp.Rows))
		for _, r := range resp.Rows {
			rows = append(rows, shared.RowToMap(r))
		}
		s.B.EmitEvent(ctx, event.RecordsAfterBulkUpsert, tableID, map[string]any{
			"rows": rows,
		})
	}
	return &resp, nil
}

func (s *Data) BulkDeleteRows(ctx context.Context, req *row.BulkDeleteRowsRequest) (*row.BulkDeleteRowsResponse, error) {
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}
	_, schemaName, tableName, err := s.meta().LoadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if len(req.RowIds) == 0 {
		return &row.BulkDeleteRowsResponse{}, nil
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
	if s.B.Events != nil && len(req.RowIds) > 0 {
		ids := make([]any, len(req.RowIds))
		for i, id := range req.RowIds {
			ids[i] = id
		}
		s.B.EmitEvent(ctx, event.RecordsAfterBulkDelete, tableID, map[string]any{
			"rowIds": ids,
		})
	}
	return &row.BulkDeleteRowsResponse{}, nil
}

func (s *Data) ExportRows(ctx context.Context, req *row.ExportRowsRequest) (*row.ExportRowsResponse, error) {
	if req.TableId == "" {
		return nil, fmt.Errorf("table_id is required")
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		return nil, fmt.Errorf("unsupported export format %q", req.Format)
	}
	qresp, err := s.QueryRows(ctx, &row.QueryRowsRequest{
		TableId:   req.TableId,
		Filter:    req.Filter,
		ColumnIds: req.ColumnIds,
		PageSize:  s.B.MaxRow,
	})
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(qresp.Rows))
	for _, r := range qresp.Rows {
		rows = append(rows, shared.RowToMap(r))
	}
	switch format {
	case "json":
		b, err := json.Marshal(rows)
		if err != nil {
			return nil, err
		}
		return &row.ExportRowsResponse{Format: "json", Content: string(b)}, nil
	case "csv":
		if len(rows) == 0 {
			return &row.ExportRowsResponse{Format: "csv", Content: ""}, nil
		}
		cols := csvColumnOrder(rows)
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		if err := w.Write(cols); err != nil {
			return nil, err
		}
		for _, row := range rows {
			rec := make([]string, len(cols))
			for i, c := range cols {
				if v, ok := row[c]; ok && v != nil {
					rec[i] = fmt.Sprint(v)
				}
			}
			if err := w.Write(rec); err != nil {
				return nil, err
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return nil, err
		}
		return &row.ExportRowsResponse{Format: "csv", Content: buf.String()}, nil
	default:
		return nil, fmt.Errorf("unsupported format")
	}
}

func csvColumnOrder(rows []map[string]any) []string {
	seen := map[string]struct{}{"id": {}}
	order := []string{"id"}
	for _, row := range rows {
		for k := range row {
			if k == "id" {
				continue
			}
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				order = append(order, k)
			}
		}
	}
	return order
}
