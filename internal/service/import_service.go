package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/webhook"
)

// ImportRows inserts rows from JSON-like structs; keys are column display names or ids unless column_map overrides.
func (s *LowcodeService) ImportRows(ctx context.Context, req *apiv1.ImportRowsRequest) (*apiv1.ImportRowsResponse, error) {
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableID := req.TableId
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}
	format := req.Format
	if format != apiv1.ImportRowsFormatUnspecified &&
		format != apiv1.ImportRowsFormatJSONRows {
		return nil, fmt.Errorf("unsupported import format %v", format)
	}
	if len(req.Rows) == 0 {
		return &apiv1.ImportRowsResponse{InsertedCount: 0}, nil
	}

	cols, schemaName, physTable, err := s.loadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no writable columns for table")
	}

	byName := make(map[string]string, len(cols))
	byID := make(map[string]columnMeta, len(cols))
	for _, c := range cols {
		byName[c.Name] = c.Id
		byID[c.Id] = c
	}
	colMap := req.ColumnMap

	tx, err := data.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var out []*apiv1.Row
	var n int32
	for _, rowMap := range req.Rows {
		if rowMap == nil {
			continue
		}
		cells, err := importRowToCells(rowMap, cols, byName, byID, colMap)
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
		out = append(out, &apiv1.Row{Id: id, Cells: cells})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.Hooks != nil && len(out) > 0 {
		rows := make([]any, 0, len(out))
		for _, r := range out {
			rows = append(rows, RowToMap(r))
		}
		s.Hooks.Emit(ctx, webhook.RecordsAfterBulkImport, tableID, map[string]any{
			"rows":          rows,
			"insertedCount": int(n),
		})
	}
	return &apiv1.ImportRowsResponse{Rows: out, InsertedCount: n}, nil
}

func importRowToCells(
	row map[string]any,
	cols []columnMeta,
	byName map[string]string,
	byID map[string]columnMeta,
	columnMap map[string]string,
) (map[string]*apiv1.Value, error) {
	cells := make(map[string]*apiv1.Value)
	for key, raw := range row {
		if key == "" {
			continue
		}
		colID, ok := columnMap[key]
		if !ok {
			if id, ok2 := byName[key]; ok2 {
				colID = id
			} else if _, ok3 := byID[key]; ok3 {
				colID = key
			} else {
				continue // unknown column, skip
			}
		}
		meta, ok := byID[colID]
		if !ok {
			return nil, fmt.Errorf("column_map references unknown column id %q", colID)
		}
		v, err := importNativeToValue(raw, meta.PgType)
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", key, err)
		}
		if v != nil {
			cells[colID] = v
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
		// text, uuid, and unknown: stringify primitives
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
