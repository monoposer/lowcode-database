package dsl

import (
	"testing"
)

func TestParseEQ(t *testing.T) {
	w, err := Parse(`{"type":"EQ","attr":"status","val":"active"}`)
	if err != nil {
		t.Fatal(err)
	}
	if w.Type != "EQ" || w.Attr != "status" || w.Val != "active" {
		t.Fatalf("unexpected: %+v", w)
	}
}

func TestParseAND(t *testing.T) {
	w, err := Parse(`{"type":"AND","val":[{"type":"EQ","attr":"a","val":1},{"type":"GT","attr":"b","val":2}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if w.Type != "AND" || len(w.Vals) != 2 {
		t.Fatalf("unexpected: %+v", w)
	}
}

func TestParseIN(t *testing.T) {
	w, err := Parse(map[string]any{
		"type": "IN",
		"attr": "id",
		"val":  []any{"a", "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if w.Type != "IN" || w.Attr != "id" {
		t.Fatalf("unexpected: %+v", w)
	}
}

func TestBuildEqualMap(t *testing.T) {
	w := BuildEqualMap(map[string]any{"x": 1, "y": "z"})
	if w.Type != "AND" || len(w.Vals) != 2 {
		t.Fatalf("expected AND with 2 children, got %+v", w)
	}
}

func TestParseEmpty(t *testing.T) {
	w, err := Parse("")
	if err != nil {
		t.Fatal(err)
	}
	if w.Type != "" {
		t.Fatalf("expected empty, got %+v", w)
	}
}

func TestParseCached(t *testing.T) {
	raw := map[string]any{"type": "EQ", "attr": "status", "val": "active"}
	w1, err := ParseCached(raw)
	if err != nil {
		t.Fatal(err)
	}
	w2, err := ParseCached(raw)
	if err != nil {
		t.Fatal(err)
	}
	if w1.Type != w2.Type || w1.Attr != w2.Attr {
		t.Fatalf("%+v vs %+v", w1, w2)
	}
}
