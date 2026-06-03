package apiv1

import "time"

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
