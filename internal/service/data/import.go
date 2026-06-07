package data

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/apiv1"
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// ImportRows inserts rows from JSON-like structs; keys are column display names or ids unless column_map overrides.
func (s *Data) ImportRows(ctx context.Context, req *row.ImportRowsRequest) (*row.ImportRowsResponse, error) {
	if err := s.checkBulkSize(len(req.Rows), "bulkImport"); err != nil {
		return nil, err
	}
	if s.B.IsRLSTableMode() {
		return nil, fmt.Errorf("rls_table mode: import not supported yet")
	}
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}
	format := req.Format
	if format != row.ImportRowsFormatUnspecified &&
		format != row.ImportRowsFormatJSONRows {
		return nil, fmt.Errorf("unsupported import format %v", format)
	}
	if len(req.Rows) == 0 {
		return &row.ImportRowsResponse{InsertedCount: 0}, nil
	}

	cols, schemaName, physTable, err := s.meta().LoadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no writable columns for table")
	}

	byName := make(map[string]shared.ColumnMeta, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
	}
	colMap := req.ColumnMap

	tx, err := data.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var n int32
	for _, rowMap := range req.Rows {
		if rowMap == nil {
			continue
		}
		cells, err := importRowToCells(rowMap, cols, byName, colMap)
		if err != nil {
			return nil, err
		}
		id, err := s.insertRowTx(ctx, tx, cols, schemaName, physTable, cells)
		if err != nil {
			return nil, err
		}
		if id == "" {
			continue
		}
		n++
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if n > 0 {
		s.B.EmitEvent(ctx, event.RecordsAfterBulkImport, tableID, map[string]any{
			"insertedCount": int(n),
		})
	}
	return &row.ImportRowsResponse{InsertedCount: n}, nil
}

// ImportRowsStream reads a JSON object with a "rows" array without buffering the whole array.
func (s *Data) ImportRowsStream(ctx context.Context, tableID string, columnMap map[string]string, r io.Reader) (*row.ImportRowsResponse, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	tok, err := dec.Token()
	if err == io.EOF {
		return &row.ImportRowsResponse{}, nil
	}
	if err != nil {
		return nil, err
	}
	d, ok := tok.(json.Delim)
	if !ok || d != '{' {
		return nil, fmt.Errorf("import body must be a JSON object")
	}
	var rows []map[string]any
	max := s.maxBulkItems()
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyTok.(string)
		switch key {
		case "columnMap", "column_map":
			if err := dec.Decode(&columnMap); err != nil {
				return nil, err
			}
		case "tableId", "table_id":
			var id string
			if err := dec.Decode(&id); err != nil {
				return nil, err
			}
			if tableID == "" {
				tableID = id
			}
		case "format":
			var skip any
			if err := dec.Decode(&skip); err != nil {
				return nil, err
			}
		case "rows":
			t, err := dec.Token()
			if err != nil {
				return nil, err
			}
			if delim, ok := t.(json.Delim); !ok || delim != '[' {
				return nil, fmt.Errorf("rows must be an array")
			}
			for dec.More() {
				var rowMap map[string]any
				if err := dec.Decode(&rowMap); err != nil {
					return nil, err
				}
				if len(rows) >= max {
					return nil, fmt.Errorf("bulkImport size exceeds hard limit %d", max)
				}
				rows = append(rows, rowMap)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
		default:
			var skip any
			if err := dec.Decode(&skip); err != nil {
				return nil, err
			}
		}
	}
	return s.ImportRows(ctx, &row.ImportRowsRequest{TableId: tableID, ColumnMap: columnMap, Rows: rows})
}

func importRowToCells(
	row map[string]any,
	cols []shared.ColumnMeta,
	byName map[string]shared.ColumnMeta,
	columnMap map[string]string,
) (map[string]*apiv1.Value, error) {
	resolveName := func(ref string) (string, bool) {
		if ref == "" {
			return "", false
		}
		if mapped, ok := columnMap[ref]; ok {
			ref = mapped
		}
		if meta, ok := byName[ref]; ok {
			return meta.Name, true
		}
		for _, c := range cols {
			if c.Id == ref {
				return c.Name, true
			}
		}
		return "", false
	}
	cells := make(map[string]*apiv1.Value)
	for key, raw := range row {
		colName, ok := resolveName(key)
		if !ok {
			continue
		}
		meta := byName[colName]
		v, err := importNativeToValue(raw, meta.PgType)
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", key, err)
		}
		if v != nil {
			cells[colName] = v
		}
	}
	return cells, nil
}

func importNativeToValue(raw interface{}, pgType string) (*apiv1.Value, error) {
	if raw == nil {
		return nil, nil
	}
	switch pgType {
	case "boolean", "bool":
		switch t := raw.(type) {
		case bool:
			return apiv1.BoolValue(t), nil
		case string:
			b, err := strconv.ParseBool(t)
			if err != nil {
				return nil, err
			}
			return apiv1.BoolValue(b), nil
		default:
			return nil, fmt.Errorf("expected bool for type %s", pgType)
		}
	case "numeric", "integer", "bigint", "bigserial", "smallint", "int", "int8", "double precision", "real":
		switch t := raw.(type) {
		case float64:
			return apiv1.NumberValue(t), nil
		case int64:
			return apiv1.NumberValue(float64(t)), nil
		case string:
			f, err := strconv.ParseFloat(t, 64)
			if err != nil {
				return nil, err
			}
			return apiv1.NumberValue(f), nil
		default:
			return nil, fmt.Errorf("expected number for type %s", pgType)
		}
	case "timestamptz", "timestamp":
		switch t := raw.(type) {
		case string:
			ts, err := time.Parse(time.RFC3339Nano, t)
			if err != nil {
				ts, err = time.Parse(time.RFC3339, t)
			}
			if err != nil {
				return nil, err
			}
			return apiv1.TimestampValue(ts), nil
		default:
			return nil, fmt.Errorf("expected RFC3339 string for timestamp")
		}
	case "date":
		switch t := raw.(type) {
		case string:
			ts, err := time.Parse("2006-01-02", t)
			if err != nil {
				return nil, err
			}
			return apiv1.TimestampValue(ts), nil
		default:
			return nil, fmt.Errorf("expected YYYY-MM-DD string for date")
		}
	case "bytea":
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("expected base64 string for bytea")
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("bytea: %w", err)
		}
		return apiv1.BytesValue(b), nil
	case "jsonb", "json":
		return apiv1.JsonValue(coerceMap(raw)), nil
	default:
		switch t := raw.(type) {
		case string:
			return apiv1.StringValue(t), nil
		case float64:
			return apiv1.StringValue(strconv.FormatFloat(t, 'f', -1, 64)), nil
		case bool:
			return apiv1.StringValue(strconv.FormatBool(t)), nil
		case map[string]interface{}:
			return apiv1.JsonValue(t), nil
		default:
			return apiv1.StringValue(fmt.Sprint(t)), nil
		}
	}
}

func coerceMap(raw interface{}) map[string]interface{} {
	if m, ok := raw.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{"value": raw}
}
