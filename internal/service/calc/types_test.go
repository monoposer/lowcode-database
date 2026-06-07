package calc

import (
	"encoding/json"
	"testing"
)

func TestUnwrapCache(t *testing.T) {
	if UnwrapCache("raw") != "raw" {
		t.Fatal("raw scalar")
	}
	w := WrapCache(120, CacheValid)
	if UnwrapCache(w) != 120 {
		t.Fatalf("got %#v", UnwrapCache(w))
	}
	if CacheStatusOf(w) != CacheValid {
		t.Fatal(CacheStatusOf(w))
	}
	b, _ := json.Marshal(w)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if UnwrapCache(m).(float64) != 120 {
		t.Fatalf("json roundtrip %#v", UnwrapCache(m))
	}
}

func TestIsLinkAndCalcType(t *testing.T) {
	if !IsLinkType("link") || !IsLinkType("relationship") || !IsLinkType("relation_fk") {
		t.Fatal("link types")
	}
	if !IsCalcType("formula") || !IsCalcType("lookup") || !IsCalcType("rollup") {
		t.Fatal("calc types")
	}
	if IsCalcType("text") || IsLinkType("formula") {
		t.Fatal("negative")
	}
}

func TestJSONBCellSQL(t *testing.T) {
	s := JSONBCellSQL("amount")
	if s == "" || len(s) < 10 {
		t.Fatal(s)
	}
}
