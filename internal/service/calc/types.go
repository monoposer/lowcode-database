package calc

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	StatusPending    int16 = 0
	StatusProcessing int16 = 1
	StatusCompleted  int16 = 2
	StatusFailed     int16 = 3

	CacheValid = "valid"
	CacheError = "error"
)

// Task is a calc_queue row.
type Task struct {
	ID             int64
	TenantID       string
	TableID        string // logical table name
	RecordID       string
	TargetFieldIDs []string
	Status         int16
	RetryCount     int
	MaxRetry       int
	VTID           string
}

// Rec is a business row snapshot.
type Rec struct {
	ID      string
	VTID    string
	WSID    string
	Data    map[string]any
	Version int64
}

// CacheCell is the formula/lookup/rollup payload stored in record.data.
type CacheCell struct {
	Value       any    `json:"value"`
	CacheStatus string `json:"_cache_status"`
}

func WrapCache(value any, status string) map[string]any {
	if status == "" {
		status = CacheValid
	}
	return map[string]any{"value": value, "_cache_status": status}
}

// UnwrapCache returns the display value. Objects with _cache_status unwrap to .value.
func UnwrapCache(v any) any {
	m, ok := asMap(v)
	if !ok {
		return v
	}
	if _, ok := m["_cache_status"]; !ok {
		return v
	}
	return m["value"]
}

func CacheStatusOf(v any) string {
	m, ok := asMap(v)
	if !ok {
		return ""
	}
	s, _ := m["_cache_status"].(string)
	return s
}

func asMap(v any) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		return t, true
	case json.RawMessage:
		var m map[string]any
		if json.Unmarshal(t, &m) == nil {
			return m, true
		}
	}
	return nil, false
}

func IsLinkType(typeID string) bool {
	switch strings.ToLower(strings.TrimSpace(typeID)) {
	case "link", "relationship", "relation_fk":
		return true
	default:
		return false
	}
}

func IsCalcType(typeID string) bool {
	switch strings.ToLower(strings.TrimSpace(typeID)) {
	case "formula", "lookup", "rollup":
		return true
	default:
		return false
	}
}

func JSONBCellSQL(field string) string {
	esc := strings.ReplaceAll(field, "'", "''")
	return fmt.Sprintf(
		`(CASE WHEN jsonb_typeof(data->'%s') = 'object' AND (data->'%s') ? 'value' THEN data#>>'{%s,value}' ELSE data->>'%s' END)`,
		esc, esc, esc, esc,
	)
}
