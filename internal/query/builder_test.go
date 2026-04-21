package query

import (
	"strings"
	"testing"

	"github.com/solat/lowcode-database/internal/dsl"
)

func TestBuildWhereEQ(t *testing.T) {
	cols := []ColumnMeta{{ID: "col1", PgColumn: "c_amount"}}
	attrMap := AttrMapFromColumns("_b", cols)
	sql, args, err := BuildWhere(dsl.Where{Type: "EQ", Attr: "col1", Val: 42}, attrMap, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "c_amount") || len(args) != 1 || args[0] != 42 {
		t.Fatalf("got sql=%q args=%v", sql, args)
	}
}

func TestBuildWhereIN(t *testing.T) {
	cols := []ColumnMeta{{ID: "col1", PgColumn: "c_status"}}
	attrMap := AttrMapFromColumns("_b", cols)
	sql, args, err := BuildWhere(dsl.Where{Type: "IN", Attr: "col1", Val: []any{"a", "b"}}, attrMap, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, " IN ") || len(args) != 2 {
		t.Fatalf("got sql=%q args=%v", sql, args)
	}
}

func TestBuildOrderBy(t *testing.T) {
	cols := []ColumnMeta{{ID: "c1", PgColumn: "c_name"}}
	attrMap := AttrMapFromColumns("_b", cols)
	sql := BuildOrderBy([]OrderSpec{{Attribute: "c1", SortOrder: "DESC"}}, attrMap, "id")
	if !strings.Contains(sql, "DESC") {
		t.Fatalf("got %q", sql)
	}
}
