package data

import (
	"testing"

	"github.com/solat/lowcode-database/internal/service/shared"
)

func TestCollectFormulaNeededRefs(t *testing.T) {
	cols := []shared.FullColumnMeta{
		{Name: "remak", Kind: "text"},
		{Name: "formula_1", Kind: "formula", Config: map[string]any{"expression": `CONCAT({{remak}}, '+1')`}},
		{Name: "formula_2", Kind: "formula", Config: map[string]any{"expression": `CONCAT({{formula_1}}, '+2')`}},
		{Name: "goods_name", Kind: "lookup", Config: map[string]any{
			"relation_column_id": "goods",
			"target_column_id":   "goods_name",
		}},
	}
	needed := collectFormulaNeededRefs("formula_2", cols)
	for _, name := range []string{"formula_2", "formula_1", "remak"} {
		if _, ok := needed[name]; !ok {
			t.Fatalf("expected %q in needed refs, got %v", name, needed)
		}
	}
	if _, ok := needed["goods_name"]; ok {
		t.Fatalf("goods_name should not be needed for formula_2, got %v", needed)
	}
}
