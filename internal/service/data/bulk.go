package data

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func (s *Data) BulkUpsertRows(ctx context.Context, req *row.BulkUpsertRowsRequest) (*row.BulkUpsertRowsResponse, error) {
	if err := s.checkBulkSize(len(req.Items), "bulkUpsert"); err != nil {
		return nil, err
	}
	if s.B.IsRLSTableMode() {
		return nil, fmt.Errorf("rls_table mode: bulkUpsert not supported yet")
	}
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
	if len(resp.Rows) > 0 {
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
	if err := s.checkBulkSize(len(req.RowIds), "bulkDelete"); err != nil {
		return nil, err
	}
	if s.B.IsRLSTableMode() {
		return nil, fmt.Errorf("rls_table mode: bulkDelete not supported yet")
	}
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
	if len(req.RowIds) > 0 {
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
	var buf bytes.Buffer
	format, err := s.ExportRowsTo(ctx, req, &buf)
	if err != nil {
		return nil, err
	}
	return &row.ExportRowsResponse{Format: format, Content: buf.String()}, nil
}

func (s *Data) ExportRowsTo(ctx context.Context, req *row.ExportRowsRequest, w io.Writer) (string, error) {
	ctx = withReadConsistency(ctx, req.Consistency)
	if req.TableId == "" {
		return "", fmt.Errorf("table_id is required")
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		return "", fmt.Errorf("unsupported export format %q", req.Format)
	}
	maxRows := s.B.MaxExportRows
	if maxRows <= 0 {
		maxRows = 100000
	}
	pageSize := s.maxScanRows()
	var token string
	exported := 0
	var csvWriter *csv.Writer
	var csvCols []string
	jsonStarted := false

	flushJSONComma := false
	for {
		if exported >= maxRows {
			break
		}
		remain := int32(maxRows - exported)
		if remain > pageSize {
			remain = pageSize
		}
		qresp, err := s.QueryRows(ctx, &row.QueryRowsRequest{
			TableId:   req.TableId,
			Filter:    req.Filter,
			ColumnIds: req.ColumnIds,
			PageSize:  remain,
			PageToken: token,
		})
		if err != nil {
			return "", err
		}
		if len(qresp.Rows) == 0 {
			break
		}
		for _, r := range qresp.Rows {
			m := shared.RowToMap(r)
			switch format {
			case "json":
				if !jsonStarted {
					if _, err := w.Write([]byte("[")); err != nil {
						return "", err
					}
					jsonStarted = true
				}
				if flushJSONComma {
					if _, err := w.Write([]byte(",")); err != nil {
						return "", err
					}
				}
				b, err := json.Marshal(m)
				if err != nil {
					return "", err
				}
				if _, err := w.Write(b); err != nil {
					return "", err
				}
				flushJSONComma = true
			case "csv":
				if csvWriter == nil {
					csvCols = csvColumnOrder([]map[string]any{m})
					csvWriter = csv.NewWriter(w)
					if err := csvWriter.Write(csvCols); err != nil {
						return "", err
					}
				}
				rec := make([]string, len(csvCols))
				for i, c := range csvCols {
					if v, ok := m[c]; ok && v != nil {
						rec[i] = fmt.Sprint(v)
					}
				}
				if err := csvWriter.Write(rec); err != nil {
					return "", err
				}
			}
			exported++
			if exported >= maxRows {
				break
			}
		}
		if qresp.NextPageToken == "" {
			break
		}
		token = qresp.NextPageToken
	}
	if format == "json" {
		if !jsonStarted {
			_, err := w.Write([]byte("[]"))
			return format, err
		}
		_, err := w.Write([]byte("]"))
		return format, err
	}
	if csvWriter != nil {
		csvWriter.Flush()
		return format, csvWriter.Error()
	}
	return format, nil
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
