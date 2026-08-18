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

func TestBuildWhereGTNumericCast(t *testing.T) {
	cols := []ColumnMeta{{ID: "score", Name: "score", PgType: "numeric"}}
	attrMap := AttrMapFromColumns("_b", cols)
	attrTypes := AttrPgTypesFromColumns(cols)
	sql, args, err := BuildWhereWithTypes(dsl.Where{Type: "GT", Attr: "score", Val: 1}, attrMap, attrTypes, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, " > $1::numeric") {
		t.Fatalf("sql: %q", sql)
	}
	if len(args) != 1 {
		t.Fatalf("args: %v", args)
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

func TestBuildWhereJSONExprNotQuotedAsIdent(t *testing.T) {
	expr := `(CASE WHEN jsonb_typeof(data->'title')='object' AND (data->'title') ? 'value' THEN data->'title'->>'value' ELSE data->>'title' END)`
	sql, _, err := BuildWhere(dsl.Where{Type: "LIKE", Attr: "title", Val: "x"}, map[string]string{"title": expr}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sql, `"(`) {
		t.Fatalf("expression was quoted as identifier: %s", sql)
	}
	if !strings.Contains(sql, "jsonb_typeof") || !strings.Contains(sql, " LIKE ") {
		t.Fatalf("sql: %s", sql)
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

func TestBuildWhereEQOnArrayUsesHas(t *testing.T) {
	cols := []ColumnMeta{{ID: "items", Name: "items", PgType: "text[]"}}
	attrMap := AttrMapFromColumns("_b", cols)
	attrTypes := AttrPgTypesFromColumns(cols)
	sql, args, err := BuildWhereWithTypes(
		dsl.Where{Type: "EQ", Attr: "items", Val: "rec-1"},
		attrMap, attrTypes, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "@> ARRAY[$1]::text[]") {
		t.Fatalf("sql: %q", sql)
	}
	if len(args) != 1 || args[0] != "rec-1" {
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

func TestBuildWhereBETWEEN(t *testing.T) {
	cols := []ColumnMeta{{ID: "created_at", Name: "created_at"}}
	attrMap := AttrMapFromColumns("_b", cols)
	sql, args, err := BuildWhere(dsl.Where{
		Type: "BETWEEN", Attr: "created_at", Val: []any{"2026-01-01", "2026-12-31"},
	}, attrMap, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, " BETWEEN $1 AND $2") {
		t.Fatalf("sql: %q", sql)
	}
	if len(args) != 2 || args[0] != "2026-01-01" || args[1] != "2026-12-31" {
		t.Fatalf("args: %v", args)
	}
}

func TestBuildWhereFTS(t *testing.T) {
	attrMap := map[string]string{"_fulltext_text": `COALESCE(data->>'_fulltext_text','')`}
	sql, args, err := BuildWhere(dsl.Where{Type: "FTS", Attr: "_fulltext_text", Val: "hello world"}, attrMap, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "to_tsvector") || !strings.Contains(sql, "to_tsquery") {
		t.Fatalf("sql: %q", sql)
	}
	if strings.Contains(sql, `"(`) {
		t.Fatalf("expr quoted as identifier: %s", sql)
	}
	if len(args) != 1 || args[0] != "hello & world" {
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
