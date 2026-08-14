package data

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// MarshalJSON emits a flat SQL-like row: { "id": "...", "col": value, ... }.
func (r Row) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, 1+len(r.Cells))
	if r.Id != "" {
		m["id"] = r.Id
	}
	if r.Version != 0 {
		m["version"] = r.Version
	}
	if r.Pending {
		m["pending"] = true
	}
	for k, v := range r.Cells {
		m[k] = shared.ValueToNative(v)
	}
	return json.Marshal(m)
}

// UnmarshalJSON accepts a flat row or { id, cells } shape.
func (r *Row) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	r.Cells = make(map[string]*shared.Value)
	if raw, ok := m["id"]; ok {
		_ = json.Unmarshal(raw, &r.Id)
	}
	if raw, ok := m["version"]; ok {
		_ = json.Unmarshal(raw, &r.Version)
	}
	if raw, ok := m["pending"]; ok {
		_ = json.Unmarshal(raw, &r.Pending)
	}
	if raw, ok := m["cells"]; ok {
		cells, err := shared.ParseCellsMap(raw)
		if err != nil {
			return err
		}
		for k, v := range cells {
			r.Cells[k] = v
		}
		return nil
	}
	skip := map[string]struct{}{"id": {}, "cells": {}, "version": {}, "pending": {}}
	cells, err := shared.ParseFlatRowFields(m, skip)
	if err != nil {
		return err
	}
	for k, v := range cells {
		r.Cells[k] = v
	}
	return nil
}

var createRowSkip = map[string]struct{}{
	"tableName": {}, "table_name": {}, "cells": {},
}

func (r *CreateRowRequest) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if raw, ok := m["tableName"]; ok {
		_ = json.Unmarshal(raw, &r.TableName)
	}
	if raw, ok := m["table_name"]; ok && r.TableName == "" {
		_ = json.Unmarshal(raw, &r.TableName)
	}
	if raw, ok := m["cells"]; ok {
		cells, err := shared.ParseCellsMap(raw)
		if err != nil {
			return err
		}
		r.Cells = cells
		return nil
	}
	cells, err := shared.ParseFlatRowFields(m, createRowSkip)
	if err != nil {
		return err
	}
	r.Cells = cells
	return nil
}

var updateRowSkip = map[string]struct{}{
	"tableName": {}, "table_name": {}, "rowId": {}, "row_id": {}, "cells": {},
}

func (r *UpdateRowRequest) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if raw, ok := m["tableName"]; ok {
		_ = json.Unmarshal(raw, &r.TableName)
	}
	if raw, ok := m["table_name"]; ok && r.TableName == "" {
		_ = json.Unmarshal(raw, &r.TableName)
	}
	if raw, ok := m["rowId"]; ok {
		_ = json.Unmarshal(raw, &r.RowId)
	}
	if raw, ok := m["row_id"]; ok && r.RowId == "" {
		_ = json.Unmarshal(raw, &r.RowId)
	}
	if raw, ok := m["cells"]; ok {
		cells, err := shared.ParseCellsMap(raw)
		if err != nil {
			return err
		}
		r.Cells = cells
		return nil
	}
	cells, err := shared.ParseFlatRowFields(m, updateRowSkip)
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
		cells, err := shared.ParseCellsMap(raw)
		if err != nil {
			return err
		}
		r.Cells = cells
		return nil
	}
	cells, err := shared.ParseFlatRowFields(m, bulkItemSkip)
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
