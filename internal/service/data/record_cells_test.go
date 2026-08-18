package data

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func TestHydrateLinkCardinality(t *testing.T) {
	cols := []shared.FullColumnMeta{
		{Name: "customer_id", TypeId: "link", Kind: "link", Config: map[string]any{"cardinality": "one", "target_table_name": "customer"}},
		{Name: "items", TypeId: "link", Kind: "link", Config: map[string]any{"cardinality": "many", "target_table_name": "order_goods"}},
	}
	links := map[string][]string{
		"customer_id": {"cust-1"},
		"items":       {"g-1", "g-2"},
	}
	cells := hydrateCells(nil, cols, links, false)

	got := shared.ValueToNative(cells["customer_id"])
	if s, ok := got.(string); !ok || s != "cust-1" {
		t.Fatalf("many-to-one customer_id=%#v want string cust-1", got)
	}

	gotItems := shared.ValueToNative(cells["items"])
	arr, ok := gotItems.([]string)
	if !ok || len(arr) != 2 || arr[0] != "g-1" || arr[1] != "g-2" {
		t.Fatalf("one-to-many items=%#v", gotItems)
	}
}

func TestHydrateLinkOneEmptyOmits(t *testing.T) {
	cols := []shared.FullColumnMeta{
		{Name: "customer_id", TypeId: "link", Kind: "link", Config: map[string]any{"cardinality": "one"}},
	}
	cells := hydrateCells(nil, cols, map[string][]string{}, false)
	if cells["customer_id"] != nil {
		t.Fatalf("empty many-to-one should omit, got %#v", cells["customer_id"])
	}
}
