// Package dsl provides a minimal filter DSL for row queries (metadata-compatible shape).
package dsl

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Where is a parsed filter node.
type Where struct {
	Type string // AND, OR, EQ, NEQ, IN, NIN, LIKE, GT, GTE, LT, LTE, EMPTY, NOT_EMPTY
	Attr string
	Val  any
	Vals []Where
}

// Parse decodes a JSON filter string or object into Where.
func Parse(raw any) (Where, error) {
	if raw == nil {
		return Where{}, nil
	}
	var b []byte
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" || s == "{}" {
			return Where{}, nil
		}
		b = []byte(s)
	case []byte:
		b = v
	default:
		var err error
		b, err = json.Marshal(v)
		if err != nil {
			return Where{}, err
		}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return Where{}, err
	}
	return parseNode(m)
}

func parseNode(m map[string]any) (Where, error) {
	if m == nil {
		return Where{}, nil
	}
	t, _ := m["type"].(string)
	t = strings.ToUpper(strings.TrimSpace(t))
	if t == "" {
		return Where{}, nil
	}
	switch t {
	case "AND", "OR":
		raw, _ := m["val"].([]any)
		var children []Where
		for _, item := range raw {
			sub, ok := item.(map[string]any)
			if !ok {
				continue
			}
			w, err := parseNode(sub)
			if err != nil {
				return Where{}, err
			}
			if w.Type != "" {
				children = append(children, w)
			}
		}
		return Where{Type: t, Vals: children}, nil
	case "EQ", "NEQ", "LIKE", "GT", "GTE", "LT", "LTE":
		attr, _ := m["attr"].(string)
		if attr == "" {
			return Where{}, fmt.Errorf("filter %s requires attr", t)
		}
		return Where{Type: t, Attr: attr, Val: m["val"]}, nil
	case "IN", "NIN":
		attr, _ := m["attr"].(string)
		if attr == "" {
			return Where{}, fmt.Errorf("filter %s requires attr", t)
		}
		return Where{Type: t, Attr: attr, Val: m["val"]}, nil
	case "EMPTY", "NOT_EMPTY", "NULL", "NOT_NULL":
		attr, _ := m["attr"].(string)
		if attr == "" {
			return Where{}, fmt.Errorf("filter %s requires attr", t)
		}
		typ := t
		if typ == "NULL" {
			typ = "EMPTY"
		}
		if typ == "NOT_NULL" {
			typ = "NOT_EMPTY"
		}
		return Where{Type: typ, Attr: attr}, nil
	default:
		return Where{}, fmt.Errorf("unsupported filter type %q", t)
	}
}

// BuildEqualMap converts a flat map to AND of EQ conditions.
func BuildEqualMap(m map[string]any) Where {
	if len(m) == 0 {
		return Where{}
	}
	var children []Where
	for k, v := range m {
		children = append(children, Where{Type: "EQ", Attr: k, Val: v})
	}
	if len(children) == 1 {
		return children[0]
	}
	return Where{Type: "AND", Vals: children}
}
