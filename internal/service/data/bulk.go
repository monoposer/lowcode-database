package data

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"github.com/monoposer/lowcode-database/internal/event"
)

func (s *Data) BulkUpsertRows(ctx context.Context, req *BulkUpsertRowsRequest) (*BulkUpsertRowsResponse, error) {
	if err := s.checkBulkSize(len(req.Items), "bulkUpsert"); err != nil {
		return nil, err
	}
	if req.TableName == "" {
		return nil, fmt.Errorf("table_name is required")
	}
	var resp BulkUpsertRowsResponse
	for _, item := range req.Items {
		if item == nil {
			continue
		}
		if item.RowId == "" {
			out, err := s.CreateRow(ctx, &CreateRowRequest{TableName: req.TableName, Cells: item.Cells})
			if err != nil {
				return nil, err
			}
			if out != nil && out.Row != nil {
				resp.Rows = append(resp.Rows, out.Row)
			}
			continue
		}
		out, err := s.UpdateRow(ctx, &UpdateRowRequest{TableName: req.TableName, RowId: item.RowId, Cells: item.Cells})
		if err != nil {
			return nil, err
		}
		if out != nil && out.Row != nil {
			resp.Rows = append(resp.Rows, out.Row)
		}
	}
	if len(resp.Rows) > 0 {
		rows := make([]any, 0, len(resp.Rows))
		for _, r := range resp.Rows {
			rows = append(rows, RowToMap(r))
		}
		s.B.EmitEvent(ctx, event.RecordsAfterBulkUpsert, req.TableName, map[string]any{"rows": rows})
	}
	return &resp, nil
}

func (s *Data) BulkDeleteRows(ctx context.Context, req *BulkDeleteRowsRequest) (*BulkDeleteRowsResponse, error) {
	if err := s.checkBulkSize(len(req.RowIds), "bulkDelete"); err != nil {
		return nil, err
	}
	if req.TableName == "" {
		return nil, fmt.Errorf("table_name is required")
	}
	if len(req.RowIds) == 0 {
		return &BulkDeleteRowsResponse{}, nil
	}
	for _, id := range req.RowIds {
		if _, err := s.DeleteRow(ctx, &DeleteRowRequest{TableName: req.TableName, RowId: id}); err != nil {
			return nil, err
		}
	}
	ids := make([]any, len(req.RowIds))
	for i, id := range req.RowIds {
		ids[i] = id
	}
	s.B.EmitEvent(ctx, event.RecordsAfterBulkDelete, req.TableName, map[string]any{"rowIds": ids})
	return &BulkDeleteRowsResponse{}, nil
}

func (s *Data) ExportRows(ctx context.Context, req *ExportRowsRequest) (*ExportRowsResponse, error) {
	var buf bytes.Buffer
	format, err := s.ExportRowsTo(ctx, req, &buf)
	if err != nil {
		return nil, err
	}
	return &ExportRowsResponse{Format: format, Content: buf.String()}, nil
}

func (s *Data) ExportRowsTo(ctx context.Context, req *ExportRowsRequest, w io.Writer) (string, error) {
	ctx = withReadConsistency(ctx, req.Consistency)
	if req.TableName == "" {
		return "", fmt.Errorf("table_name is required")
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
		qresp, err := s.QueryRows(ctx, &QueryRowsRequest{
			TableName:   req.TableName,
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
			m := RowToMap(r)
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
