package apiv1

import (
	"encoding/json"
	"fmt"
	"time"
)

// Value is a cell value in the JSON API (same union shape as the former proto Value).
type Value struct {
	StringValue    *string    `json:"stringValue,omitempty"`
	NumberValue    *float64   `json:"numberValue,omitempty"`
	BoolValue      *bool      `json:"boolValue,omitempty"`
	TimestampValue *time.Time `json:"timestampValue,omitempty"`
	BytesValue     []byte     `json:"bytesValue,omitempty"`
	JsonValue      any        `json:"jsonValue,omitempty"`
}

func StringValue(s string) *Value {
	return &Value{StringValue: &s}
}

func NumberValue(f float64) *Value {
	return &Value{NumberValue: &f}
}

func BoolValue(b bool) *Value {
	return &Value{BoolValue: &b}
}

func TimestampValue(t time.Time) *Value {
	if t.IsZero() {
		return nil
	}
	tt := t
	return &Value{TimestampValue: &tt}
}

func BytesValue(b []byte) *Value {
	if b == nil {
		return nil
	}
	return &Value{BytesValue: b}
}

func JsonValue(v any) *Value {
	return &Value{JsonValue: v}
}

func ValueString(v *Value) string {
	if v == nil || v.StringValue == nil {
		return ""
	}
	return *v.StringValue
}

func ValueNumber(v *Value) float64 {
	if v == nil || v.NumberValue == nil {
		return 0
	}
	return *v.NumberValue
}

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

// UnmarshalCellRaw decodes one JSON cell value into a typed Value.
func UnmarshalCellRaw(raw json.RawMessage) (*Value, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var anyVal any
	if err := json.Unmarshal(raw, &anyVal); err != nil {
		return nil, err
	}
	return NativeToValue(anyVal), nil
}

// ParseFlatRowFields extracts cell values from a flat row JSON object.
func ParseFlatRowFields(m map[string]json.RawMessage, skipKeys map[string]struct{}) (map[string]*Value, error) {
	cells := make(map[string]*Value)
	for k, raw := range m {
		if skipKeys != nil {
			if _, skip := skipKeys[k]; skip {
				continue
			}
		}
		v, err := UnmarshalCellRaw(raw)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", k, err)
		}
		if v != nil {
			cells[k] = v
		}
	}
	return cells, nil
}

// ParseLegacyCells decodes the legacy { "cells": { ... } } shape.
func ParseLegacyCells(raw json.RawMessage) (map[string]*Value, error) {
	var cells map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cells); err != nil {
		return nil, err
	}
	out := make(map[string]*Value, len(cells))
	for k, cv := range cells {
		v, err := UnmarshalCellRaw(cv)
		if err != nil {
			return nil, err
		}
		if v != nil {
			out[k] = v
		}
	}
	return out, nil
}
