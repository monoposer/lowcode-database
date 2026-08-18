package shared

import (
	"fmt"
	"maps"
	"regexp"
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
	rel := CfgString(cfg, "relation_column_id")
	if rel == "" {
		rel = CfgString(cfg, "link_field_id")
	}
	if rel == "" {
		return fmt.Errorf("rollup config requires link_field_id (or relation_column_id)")
	}
	agg := CfgString(cfg, "aggregate")
	if agg == "" {
		agg = CfgString(cfg, "aggregation")
	}
	if agg == "" {
		return fmt.Errorf("rollup config requires aggregation (sum|count|min|max|avg)")
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

// ValidateLinkedFilter checks optional filter on lookup/rollup config (same DSL as saved query filter).
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

// InverseLinkCardinality returns the Teable-style opposite side for one|many.
// one ↔ many; many-many / one-one keep the same when inverse_cardinality is set explicitly.
func InverseLinkCardinality(card string) string {
	switch strings.ToLower(strings.TrimSpace(card)) {
	case "one":
		return "many"
	default:
		return "one"
	}
}

// LinkInverseFieldKey is the link_ref from_field_id for the symmetric field (prefer name).
func LinkInverseFieldKey(cfg map[string]any) string {
	if name := CfgString(cfg, "inverse_field_name"); name != "" {
		return name
	}
	return CfgString(cfg, "inverse_field_id")
}

// NormalizeRelationshipConfig validates relationship column config.
func NormalizeRelationshipConfig(cfg map[string]any) (map[string]any, error) {
	if cfg == nil {
		cfg = map[string]any{}
	}
	out := maps.Clone(cfg)
	delete(out, "_skip_inverse")
	delete(out, "_skip_inverse_delete")
	if CfgString(out, "target_table_name") == "" {
		if t := CfgString(out, "to_table_name"); t != "" {
			out["target_table_name"] = t
		}
	}
	if CfgString(out, "to_table_name") == "" && CfgString(out, "target_table_name") != "" {
		out["to_table_name"] = CfgString(out, "target_table_name")
	}
	targetTable := CfgString(out, "target_table_name")
	if targetTable == "" {
		return nil, fmt.Errorf("link/relationship config requires to_table_name (or target_table_name)")
	}
	linkID := CfgString(out, "link_column_id")
	targetColID := CfgString(out, "target_column_id")
	card := strings.ToLower(CfgString(out, "cardinality"))

	// link_ref model: only target table is required; cardinality defaults to many.
	// Teable-style two-way links default to bidirectional=true when omitted.
	if linkID == "" && targetColID == "" {
		if card == "one" {
			out["cardinality"] = "one"
		} else {
			out["cardinality"] = "many"
		}
		if _, ok := out["bidirectional"]; !ok {
			out["bidirectional"] = true
		}
		if invCard := strings.ToLower(CfgString(out, "inverse_cardinality")); invCard == "one" || invCard == "many" {
			out["inverse_cardinality"] = invCard
		} else if CfgBool(out, "bidirectional") {
			out["inverse_cardinality"] = InverseLinkCardinality(CfgString(out, "cardinality"))
		}
		return out, nil
	}
	if linkID != "" && targetColID != "" {
		return nil, fmt.Errorf("relationship config: set only one of link_column_id (many) or target_column_id (one), not both")
	}
	if linkID != "" {
		if card == "one" {
			return nil, fmt.Errorf("relationship cardinality one requires target_column_id, not link_column_id")
		}
		out["cardinality"] = "many"
		delete(out, "target_column_id")
		if _, ok := out["bidirectional"]; !ok {
			out["bidirectional"] = false
		}
		return out, nil
	}
	if card == "many" {
		return nil, fmt.Errorf("relationship cardinality many requires link_column_id")
	}
	out["cardinality"] = "one"
	delete(out, "link_column_id")
	if _, ok := out["bidirectional"]; !ok {
		out["bidirectional"] = false
	}
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

var pgObjectNameRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

func ValidateTableName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if !pgObjectNameRe.MatchString(name) {
		return fmt.Errorf("name must match %s (English identifier, PG-compatible)", pgObjectNameRe.String())
	}
	return nil
}

func ValidateColumnName(name string) error {
	if err := ValidateTableName(name); err != nil {
		return err
	}
	if strings.EqualFold(name, "id") {
		return fmt.Errorf("column name %q is reserved (row primary key)", name)
	}
	return nil
}
