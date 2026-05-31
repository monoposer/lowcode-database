package apiv1

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ValueToNative converts a typed cell Value to plain JSON (SQL-like scalars).
func ValueToNative(v *Value) any {
	if v == nil {
		return nil
	}
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.NumberValue != nil {
		return *v.NumberValue
	}
	if v.BoolValue != nil {
		return *v.BoolValue
	}
	if v.TimestampValue != nil {
		return v.TimestampValue.Format(time.RFC3339Nano)
	}
	if v.BytesValue != nil {
		return v.BytesValue
	}
	if v.JsonValue != nil {
		return v.JsonValue
	}
	return nil
}

// NativeToValue converts a plain JSON value into a typed cell Value.
func NativeToValue(raw any) *Value {
	if raw == nil {
		return nil
	}
	switch t := raw.(type) {
	case string:
		if ts, err := time.Parse(time.RFC3339Nano, t); err == nil {
			return TimestampValue(ts)
		}
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return TimestampValue(ts)
		}
		return StringValue(t)
	case float64:
		return NumberValue(t)
	case bool:
		return BoolValue(t)
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return NumberValue(float64(i))
		}
		if f, err := t.Float64(); err == nil {
			return NumberValue(f)
		}
	case map[string]any:
		// Legacy typed wrapper: { "stringValue": "x" }
		if isTypedValueMap(t) {
			return decodeTypedValueMap(t)
		}
		return JsonValue(t)
	case []any:
		return JsonValue(t)
	default:
		return StringValue(fmt.Sprint(t))
	}
	return nil
}

func isTypedValueMap(m map[string]any) bool {
	for k := range m {
		switch k {
		case "stringValue", "numberValue", "boolValue", "timestampValue", "bytesValue", "jsonValue":
			return true
		}
	}
	return false
}

func decodeTypedValueMap(m map[string]any) *Value {
	v := &Value{}
	if s, ok := m["stringValue"].(string); ok {
		v.StringValue = &s
	}
	if n, ok := m["numberValue"].(float64); ok {
		v.NumberValue = &n
	}
	if b, ok := m["boolValue"].(bool); ok {
		v.BoolValue = &b
	}
	if s, ok := m["timestampValue"].(string); ok {
		if ts, err := time.Parse(time.RFC3339Nano, s); err == nil {
			v.TimestampValue = &ts
		} else if ts, err := time.Parse(time.RFC3339, s); err == nil {
			v.TimestampValue = &ts
		}
	}
	if j, ok := m["jsonValue"]; ok {
		v.JsonValue = j
	}
	return v
}

func unmarshalCellRaw(raw json.RawMessage) (*Value, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var anyVal any
	if err := json.Unmarshal(raw, &anyVal); err != nil {
		return nil, err
	}
	return NativeToValue(anyVal), nil
}

func parseFlatRowFields(m map[string]json.RawMessage, skipKeys map[string]struct{}) (map[string]*Value, error) {
	cells := make(map[string]*Value)
	for k, raw := range m {
		if skipKeys != nil {
			if _, skip := skipKeys[k]; skip {
				continue
			}
		}
		v, err := unmarshalCellRaw(raw)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", k, err)
		}
		if v != nil {
			cells[k] = v
		}
	}
	return cells, nil
}

func parseLegacyCells(raw json.RawMessage) (map[string]*Value, error) {
	var cells map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cells); err != nil {
		return nil, err
	}
	out := make(map[string]*Value, len(cells))
	for k, cv := range cells {
		v, err := unmarshalCellRaw(cv)
		if err != nil {
			return nil, err
		}
		if v != nil {
			out[k] = v
		}
	}
	return out, nil
}

// MarshalJSON emits a flat SQL-like row: { "id": "...", "col": value, ... }.
func (r Row) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, 1+len(r.Cells))
	if r.Id != "" {
		m["id"] = r.Id
	}
	for k, v := range r.Cells {
		m[k] = ValueToNative(v)
	}
	return json.Marshal(m)
}

// UnmarshalJSON accepts a flat row or legacy { id, cells } shape.
func (r *Row) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	r.Cells = make(map[string]*Value)
	if raw, ok := m["id"]; ok {
		_ = json.Unmarshal(raw, &r.Id)
	}
	if raw, ok := m["cells"]; ok {
		cells, err := parseLegacyCells(raw)
		if err != nil {
			return err
		}
		for k, v := range cells {
			r.Cells[k] = v
		}
		return nil
	}
	skip := map[string]struct{}{"id": {}, "cells": {}}
	cells, err := parseFlatRowFields(m, skip)
	if err != nil {
		return err
	}
	for k, v := range cells {
		r.Cells[k] = v
	}
	return nil
}

var createRowSkip = map[string]struct{}{
	"tableId": {}, "table_id": {}, "cells": {},
}

func (r *CreateRowRequest) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if raw, ok := m["tableId"]; ok {
		_ = json.Unmarshal(raw, &r.TableId)
	}
	if raw, ok := m["table_id"]; ok && r.TableId == "" {
		_ = json.Unmarshal(raw, &r.TableId)
	}
	if raw, ok := m["cells"]; ok {
		cells, err := parseLegacyCells(raw)
		if err != nil {
			return err
		}
		r.Cells = cells
		return nil
	}
	cells, err := parseFlatRowFields(m, createRowSkip)
	if err != nil {
		return err
	}
	r.Cells = cells
	return nil
}

var updateRowSkip = map[string]struct{}{
	"tableId": {}, "table_id": {}, "rowId": {}, "row_id": {}, "cells": {},
}

func (r *UpdateRowRequest) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if raw, ok := m["tableId"]; ok {
		_ = json.Unmarshal(raw, &r.TableId)
	}
	if raw, ok := m["table_id"]; ok && r.TableId == "" {
		_ = json.Unmarshal(raw, &r.TableId)
	}
	if raw, ok := m["rowId"]; ok {
		_ = json.Unmarshal(raw, &r.RowId)
	}
	if raw, ok := m["row_id"]; ok && r.RowId == "" {
		_ = json.Unmarshal(raw, &r.RowId)
	}
	if raw, ok := m["cells"]; ok {
		cells, err := parseLegacyCells(raw)
		if err != nil {
			return err
		}
		r.Cells = cells
		return nil
	}
	cells, err := parseFlatRowFields(m, updateRowSkip)
	if err != nil {
		return err
	}
	r.Cells = cells
	return nil
}

var bulkItemSkip = map[string]struct{}{
	"rowId": {}, "row_id": {}, "cells": {},
}

func (r *BulkUpsertRowItem) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if raw, ok := m["rowId"]; ok {
		_ = json.Unmarshal(raw, &r.RowId)
	}
	if raw, ok := m["row_id"]; ok && r.RowId == "" {
		_ = json.Unmarshal(raw, &r.RowId)
	}
	if raw, ok := m["cells"]; ok {
		cells, err := parseLegacyCells(raw)
		if err != nil {
			return err
		}
		r.Cells = cells
		return nil
	}
	cells, err := parseFlatRowFields(m, bulkItemSkip)
	if err != nil {
		return err
	}
	r.Cells = cells
	return nil
}

// ParseRowID extracts row id from a flat map (import / ad-hoc).
func ParseRowID(m map[string]any) string {
	if v, ok := m["id"]; ok && v != nil {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			return strconv.FormatInt(int64(t), 10)
		default:
			return fmt.Sprint(t)
		}
	}
	return ""
}
