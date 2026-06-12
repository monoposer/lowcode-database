package row

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/solat/lowcode-database/internal/apiv1"
)

// MarshalJSON emits a flat SQL-like row: { "id": "...", "col": value, ... }.
func (r Row) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, 1+len(r.Cells))
	if r.Id != "" {
		m["id"] = r.Id
	}
	for k, v := range r.Cells {
		m[k] = apiv1.ValueToNative(v)
	}
	return json.Marshal(m)
}

// UnmarshalJSON accepts a flat row or legacy { id, cells } shape.
func (r *Row) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	r.Cells = make(map[string]*apiv1.Value)
	if raw, ok := m["id"]; ok {
		_ = json.Unmarshal(raw, &r.Id)
	}
	if raw, ok := m["cells"]; ok {
		cells, err := apiv1.ParseLegacyCells(raw)
		if err != nil {
			return err
		}
		for k, v := range cells {
			r.Cells[k] = v
		}
		return nil
	}
	skip := map[string]struct{}{"id": {}, "cells": {}}
	cells, err := apiv1.ParseFlatRowFields(m, skip)
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
		cells, err := apiv1.ParseLegacyCells(raw)
		if err != nil {
			return err
		}
		r.Cells = cells
		return nil
	}
	cells, err := apiv1.ParseFlatRowFields(m, createRowSkip)
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
		cells, err := apiv1.ParseLegacyCells(raw)
		if err != nil {
			return err
		}
		r.Cells = cells
		return nil
	}
	cells, err := apiv1.ParseFlatRowFields(m, updateRowSkip)
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
		cells, err := apiv1.ParseLegacyCells(raw)
		if err != nil {
			return err
		}
		r.Cells = cells
		return nil
	}
	cells, err := apiv1.ParseFlatRowFields(m, bulkItemSkip)
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
