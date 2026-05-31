package shared

import (
	"fmt"
	"maps"
	"strings"
)

func CfgString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	v, ok := cfg[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func CfgBool(cfg map[string]any, key string) bool {
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
		return t == "true" || t == "1"
	default:
		return false
	}
}

func NullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func ValidateRollupConfig(cfg map[string]any) error {
	if CfgString(cfg, "relation_column_id") == "" {
		return fmt.Errorf("rollup config requires relation_column_id")
	}
	agg := CfgString(cfg, "aggregate")
	if agg == "" {
		return fmt.Errorf("rollup config requires aggregate (sum|count|min|max|avg)")
	}
	switch agg {
	case "sum", "count", "min", "max", "avg", "SUM", "COUNT", "MIN", "MAX", "AVG":
		if err := ValidateLinkedFilter(cfg); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("rollup aggregate %q not supported", agg)
	}
}

// ValidateLinkedFilter checks optional filter on lookup/rollup config (same DSL as datasource filter).
func ValidateLinkedFilter(cfg map[string]any) error {
	raw, ok := cfg["filter"]
	if !ok || raw == nil {
		return nil
	}
	if _, ok := raw.(map[string]any); !ok {
		return fmt.Errorf("filter must be a JSON object")
	}
	return nil
}

// NormalizeRelationshipConfig validates relationship column config.
func NormalizeRelationshipConfig(cfg map[string]any) (map[string]any, error) {
	if cfg == nil {
		cfg = map[string]any{}
	}
	out := maps.Clone(cfg)
	targetTable := CfgString(out, "target_table_id")
	if targetTable == "" {
		return nil, fmt.Errorf("relationship config requires target_table_id")
	}
	linkID := CfgString(out, "link_column_id")
	targetColID := CfgString(out, "target_column_id")
	card := strings.ToLower(CfgString(out, "cardinality"))

	if linkID != "" && targetColID != "" {
		return nil, fmt.Errorf("relationship config: set only one of link_column_id (many) or target_column_id (one), not both")
	}
	if linkID == "" && targetColID == "" {
		return nil, fmt.Errorf("relationship config requires link_column_id (many) or target_column_id (one)")
	}
	if linkID != "" {
		if card == "one" {
			return nil, fmt.Errorf("relationship cardinality one requires target_column_id, not link_column_id")
		}
		out["cardinality"] = "many"
		delete(out, "target_column_id")
		return out, nil
	}
	if card == "many" {
		return nil, fmt.Errorf("relationship cardinality many requires link_column_id")
	}
	out["cardinality"] = "one"
	delete(out, "link_column_id")
	return out, nil
}

func EffectiveRelationshipCardinality(cfg map[string]any, linkID, targetColID string) string {
	if c := strings.ToLower(CfgString(cfg, "cardinality")); c == "one" || c == "many" {
		return c
	}
	if linkID != "" && targetColID != "" {
		return "many"
	}
	if linkID != "" {
		return "many"
	}
	return "one"
}
