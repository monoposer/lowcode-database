package service

import (
	"testing"
)

func TestNormalizeRelationshipConfigMany(t *testing.T) {
	cfg, err := NormalizeRelationshipConfig(map[string]any{
		"target_table_id": "orders",
		"link_column_id":  "col-uuid",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfgString(cfg, "cardinality") != "many" {
		t.Fatalf("expected many, got %q", cfgString(cfg, "cardinality"))
	}
}

func TestNormalizeRelationshipConfigOne(t *testing.T) {
	cfg, err := NormalizeRelationshipConfig(map[string]any{
		"target_table_id":  "vendor",
		"target_column_id": "fk-col",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfgString(cfg, "cardinality") != "one" {
		t.Fatalf("expected one, got %q", cfgString(cfg, "cardinality"))
	}
}

func TestNormalizeRelationshipConfigConflict(t *testing.T) {
	_, err := NormalizeRelationshipConfig(map[string]any{
		"target_table_id":  "vendor",
		"link_column_id":   "a",
		"target_column_id": "b",
	})
	if err == nil {
		t.Fatal("expected error when both link and target set")
	}
}

func TestValidateRollupConfig(t *testing.T) {
	if err := validateRollupConfig(map[string]any{
		"relation_column_id": "rel",
		"aggregate":          "sum",
	}); err != nil {
		t.Fatal(err)
	}
	if err := validateRollupConfig(map[string]any{"relation_column_id": "rel"}); err == nil {
		t.Fatal("expected missing aggregate error")
	}
}

func TestEffectivePgType(t *testing.T) {
	got := effectivePgType("numeric", map[string]any{"precision": float64(20), "scale": float64(4)})
	if got != "numeric(20,4)" {
		t.Fatalf("got %q", got)
	}
}

func TestCfgBool(t *testing.T) {
	if !cfgBool(map[string]any{"add_fk": true}, "add_fk") {
		t.Fatal("expected true")
	}
	if cfgBool(map[string]any{"add_fk": "true"}, "add_fk") {
		// string true is supported
	} else {
		t.Fatal("expected string true")
	}
}
