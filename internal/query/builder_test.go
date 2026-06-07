package query

import (
	"strings"
	"testing"

	"github.com/monoposer/lowcode-database/internal/dsl"
)

func TestBuildWhereEQ(t *testing.T) {
	cols := []ColumnMeta{{ID: "col1", Name: "amount"}}
	attrMap := AttrMapFromColumns("_b", cols)
	sql, args, err := BuildWhere(dsl.Where{Type: "EQ", Attr: "col1", Val: 42}, attrMap, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "amount") || len(args) != 1 || args[0] != 42 {
		t.Fatalf("got sql=%q args=%v", sql, args)
	}
}

func TestBuildWhereIN(t *testing.T) {
	cols := []ColumnMeta{{ID: "col1", Name: "status"}}
	attrMap := AttrMapFromColumns("_b", cols)
	sql, args, err := BuildWhere(dsl.Where{Type: "IN", Attr: "col1", Val: []any{"a", "b"}}, attrMap, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, " IN ") || len(args) != 2 {
		t.Fatalf("got sql=%q args=%v", sql, args)
	}
}

func TestBuildWhereLIKEContains(t *testing.T) {
	cols := []ColumnMeta{{ID: "name", Name: "name"}}
	attrMap := AttrMapFromColumns("_b", cols)
	sql, args, err := BuildWhere(dsl.Where{Type: "LIKE", Attr: "name", Val: "客"}, attrMap, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, " LIKE ") {
		t.Fatalf("sql: %q", sql)
	}
	if len(args) != 1 || args[0] != "%客%" {
		t.Fatalf("args: %v", args)
	}
}

func TestBuildWhereArrayHas(t *testing.T) {
	cols := []ColumnMeta{{ID: "tags", Name: "multi_select", PgType: "text[]"}}
	attrMap := AttrMapFromColumns("_b", cols)
	attrTypes := AttrPgTypesFromColumns(cols)
	sql, args, err := BuildWhereWithTypes(
		dsl.Where{Type: "ARRAY_HAS", Attr: "tags", Val: "数据1"},
		attrMap, attrTypes, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "@> ARRAY[$1]::text[]") {
		t.Fatalf("sql: %q", sql)
	}
	if len(args) != 1 || args[0] != "数据1" {
		t.Fatalf("args: %v", args)
	}
}

func TestBuildWhereArrayOverlap(t *testing.T) {
	cols := []ColumnMeta{{ID: "tags", Name: "multi_select", PgType: "text[]"}}
	attrMap := AttrMapFromColumns("_b", cols)
	attrTypes := AttrPgTypesFromColumns(cols)
	sql, args, err := BuildWhereWithTypes(
		dsl.Where{Type: "ARRAY_OVERLAP", Attr: "tags", Val: []any{"a", "b"}},
		attrMap, attrTypes, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, " && ") || !strings.Contains(sql, "text[]") {
		t.Fatalf("sql: %q", args)
	}
	if len(args) != 1 {
		t.Fatalf("args: %v", args)
	}
}

func TestBuildWhereArrayHasVirtualLookup(t *testing.T) {
	subquery := `(SELECT COALESCE(array_agg(_r."name"), '{}'::text[]) FROM child _r WHERE _r.order_id = _b.id)`
	attrMap := map[string]string{"goods_name": subquery}
	attrTypes := map[string]string{"goods_name": "text[]"}
	sql, args, err := BuildWhereWithTypes(
		dsl.Where{Type: "ARRAY_HAS", Attr: "goods_name", Val: "商品"},
		attrMap, attrTypes, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "@> ARRAY[$1]::text[]") {
		t.Fatalf("sql: %q", sql)
	}
	if len(args) != 1 || args[0] != "商品" {
		t.Fatalf("args: %v", args)
	}
}

func TestBuildWhereLIKEOnArrayColumn(t *testing.T) {
	cols := []ColumnMeta{{ID: "tags", Name: "multi_select", PgType: "text[]"}}
	attrMap := AttrMapFromColumns("_b", cols)
	attrTypes := AttrPgTypesFromColumns(cols)
	sql, _, err := BuildWhereWithTypes(
		dsl.Where{Type: "LIKE", Attr: "tags", Val: "数据1"},
		attrMap, attrTypes, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sql, " LIKE ") {
		t.Fatalf("expected @> not LIKE, sql: %q", sql)
	}
	if !strings.Contains(sql, "@> ARRAY[$1]::text[]") {
		t.Fatalf("sql: %q", sql)
	}
}

func TestBuildWhereArrayNotHas(t *testing.T) {
	cols := []ColumnMeta{{ID: "tags", Name: "multi_select", PgType: "text[]"}}
	attrMap := AttrMapFromColumns("_b", cols)
	attrTypes := AttrPgTypesFromColumns(cols)
	sql, args, err := BuildWhereWithTypes(
		dsl.Where{Type: "ARRAY_NOT_HAS", Attr: "tags", Val: "数据1"},
		attrMap, attrTypes, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "NOT (") || !strings.Contains(sql, "@> ARRAY[$1]::text[]") {
		t.Fatalf("sql: %q", sql)
	}
	if len(args) != 1 || args[0] != "数据1" {
		t.Fatalf("args: %v", args)
	}
}

func TestBuildOrderBy(t *testing.T) {
	cols := []ColumnMeta{{ID: "c1", Name: "name"}}
	attrMap := AttrMapFromColumns("_b", cols)
	sql := BuildOrderBy([]OrderSpec{{Attribute: "c1", SortOrder: "DESC"}}, attrMap, "id")
	if !strings.Contains(sql, "DESC") {
		t.Fatalf("got %q", sql)
	}
}
