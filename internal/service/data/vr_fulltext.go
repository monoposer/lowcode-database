package data

import (
	"context"
	"fmt"
	"strings"
)

// applyFulltextOnWrite builds data._fulltext_text from columns with enable_fulltext.
func (s *Data) applyFulltextOnWrite(ctx context.Context, tableName string, _ interface{}, native map[string]any) map[string]any {
	if native == nil {
		native = map[string]any{}
	}
	all, _, _, err := s.meta().LoadAllColumnMeta(ctx, tableName)
	if err != nil {
		return native
	}
	var parts []string
	seen := map[string]bool{}
	for _, c := range all {
		if !configBool(c.Config, "enable_fulltext") {
			continue
		}
		if v, ok := native[c.Name]; ok && v != nil {
			s := fmt.Sprint(v)
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		delete(native, "_fulltext_text")
		return native
	}
	native["_fulltext_text"] = strings.Join(parts, " ")
	return native
}

func configBool(cfg map[string]any, key string) bool {
	if cfg == nil {
		return false
	}
	v, ok := cfg[key]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	default:
		return false
	}
}

func configString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	v, ok := cfg[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
