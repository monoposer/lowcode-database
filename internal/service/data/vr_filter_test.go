package data

import (
	"strings"
	"testing"

	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func TestVRFilterSQL_LinkContainsID(t *testing.T) {
	cols := []shared.ColumnMeta{{Name: "items", TypeId: "link"}}
	argN := 3
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "LIKE",
		"attr": "items",
		"val":  "98ba2647-ffa2-419f-8c2b-fafac6a449be",
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("preds: %v", preds)
	}
	sql := preds[0]
	if strings.Contains(sql, "data->") || strings.Contains(sql, " LIKE ") {
		t.Fatalf("link filter must use link_ref, got: %s", sql)
	}
	if !strings.Contains(sql, "array_agg(lr.to_record_id)") || !strings.Contains(sql, "from_field_id") {
		t.Fatalf("sql: %s", sql)
	}
	if !strings.Contains(sql, "@> ARRAY[$3]::text[]") {
		t.Fatalf("sql: %s", sql)
	}
	if len(args) != 1 || args[0] != "98ba2647-ffa2-419f-8c2b-fafac6a449be" {
		t.Fatalf("args: %v", args)
	}
}

func TestVRFilterSQL_LinkEmpty(t *testing.T) {
	cols := []shared.ColumnMeta{{Name: "items", TypeId: "link"}}
	argN := 1
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "EMPTY",
		"attr": "items",
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preds[0], "link_ref") || !strings.Contains(preds[0], "'{}'::text[]") {
		t.Fatalf("sql: %s", preds[0])
	}
}

func TestVRFilterSQL_ArrayHasDSL(t *testing.T) {
	cols := []shared.ColumnMeta{
		{Id: "tags", Name: "tags", TypeId: "text", PgType: "text[]"},
	}
	argN := 3
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "ARRAY_HAS",
		"attr": "tags",
		"val":  "urgent",
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("preds: %v", preds)
	}
	if !strings.Contains(preds[0], "@> ARRAY[$3]::text[]") {
		t.Fatalf("sql: %s", preds[0])
	}
	if !strings.Contains(preds[0], "jsonb_array_elements_text") {
		t.Fatalf("expected jsonb→array expr, got: %s", preds[0])
	}
	if len(args) != 1 || args[0] != "urgent" {
		t.Fatalf("args: %v", args)
	}
	if argN != 4 {
		t.Fatalf("argN=%d", argN)
	}
}

func TestVRFilterSQL_ArrayOverlapAndGroup(t *testing.T) {
	cols := []shared.ColumnMeta{
		{Name: "tags", PgType: "text[]"},
		{Name: "title", PgType: "text"},
	}
	argN := 1
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "AND",
		"val": []any{
			map[string]any{"type": "ARRAY_OVERLAP", "attr": "tags", "val": []any{"a", "b"}},
			map[string]any{"type": "LIKE", "attr": "title", "val": "hello"},
		},
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("preds: %v", preds)
	}
	sql := preds[0]
	if !strings.Contains(sql, " && ") || !strings.Contains(sql, " LIKE ") {
		t.Fatalf("sql: %s", sql)
	}
	if len(args) != 2 {
		t.Fatalf("args: %v", args)
	}
}

func TestVRFilterSQL_EQ(t *testing.T) {
	cols := []shared.ColumnMeta{{Name: "title", PgType: "text"}}
	argN := 1
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "EQ",
		"attr": "title",
		"val":  "x",
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 || !strings.Contains(preds[0], " = $1") {
		t.Fatalf("preds=%v", preds)
	}
	if len(args) != 1 || args[0] != "x" {
		t.Fatalf("args: %v", args)
	}
}

func TestVRFilterSQL_RejectsLegacy(t *testing.T) {
	cols := []shared.ColumnMeta{{Name: "title", PgType: "text"}}
	argN := 1
	var args []any
	_, err := vrFilterSQL(map[string]any{
		"field": "title",
		"op":    "eq",
		"value": "x",
	}, cols, &argN, &args)
	if err == nil {
		t.Fatal("expected error for legacy filter")
	}
}

func TestVRFilterSQL_FTS(t *testing.T) {
	argN := 1
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "FTS",
		"val":  "hello world",
	}, nil, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 || !strings.Contains(preds[0], "to_tsvector") || !strings.Contains(preds[0], "_fulltext_text") {
		t.Fatalf("preds=%v", preds)
	}
	if len(args) != 1 || args[0] != "hello & world" {
		t.Fatalf("args: %v", args)
	}
}

func TestVRFilterSQL_INRecordIDs(t *testing.T) {
	argN := 1
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "IN",
		"attr": "id",
		"val":  []any{"a", "b"},
	}, nil, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 || !strings.Contains(preds[0], "record_id") || !strings.Contains(preds[0], " IN ") {
		t.Fatalf("preds=%v", preds)
	}
	if len(args) != 2 {
		t.Fatalf("args: %v", args)
	}
}

func TestVRFilterSQL_CreatedAtEmptyUsesPhysicalColumn(t *testing.T) {
	cols := []shared.ColumnMeta{{Name: "created_at", TypeId: "datetime", PgType: "timestamptz"}}
	argN := 1
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "EMPTY",
		"attr": "created_at",
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("preds: %v", preds)
	}
	sql := preds[0]
	if strings.Contains(sql, "data->") {
		t.Fatalf("created_at EMPTY must use physical column, got: %s", sql)
	}
	if !strings.Contains(sql, "created_at") || !strings.Contains(sql, " IS NULL") {
		t.Fatalf("sql: %s", sql)
	}

	preds, err = vrFilterSQL(map[string]any{
		"type": "NOT_EMPTY",
		"attr": "created_at",
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preds[0], " IS NOT NULL") || strings.Contains(preds[0], "data->") {
		t.Fatalf("sql: %s", preds[0])
	}
}

func TestVRFilterSQL_BETWEENDatetime(t *testing.T) {
	cols := []shared.ColumnMeta{{Name: "created_at", PgType: "timestamptz"}}
	argN := 1
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "BETWEEN",
		"attr": "created_at",
		"val":  []any{"2026-01-01T00:00", "2026-12-31T23:59"},
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("preds: %v", preds)
	}
	sql := preds[0]
	if strings.Contains(sql, "data->") || strings.Contains(sql, `"(`) {
		t.Fatalf("created_at filter must use physical column, got: %s", sql)
	}
	if !strings.Contains(sql, "created_at") || !strings.Contains(sql, " BETWEEN $1 AND $2") {
		t.Fatalf("sql: %s", sql)
	}
	if len(args) != 2 {
		t.Fatalf("args: %v", args)
	}
}

func TestVRFilterSQL_IDContainsUsesRecordID(t *testing.T) {
	cols := []shared.ColumnMeta{
		{Name: "id", TypeId: "text", PgType: "text"},
		{Name: "title", PgType: "text"},
	}
	argN := 1
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "LIKE",
		"attr": "id",
		"val":  "3438",
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("preds: %v", preds)
	}
	sql := preds[0]
	if strings.Contains(sql, "data->") || strings.Contains(sql, `"(`) {
		t.Fatalf("id filter must use record_id, got: %s", sql)
	}
	if !strings.Contains(sql, "record_id") || !strings.Contains(sql, " LIKE ") {
		t.Fatalf("sql: %s", sql)
	}
	if len(args) != 1 || args[0] != "%3438%" {
		t.Fatalf("args: %v", args)
	}
}

func TestVRFilterSQL_NumberCompare(t *testing.T) {
	cols := []shared.ColumnMeta{{Name: "score", PgType: "numeric"}}
	argN := 1
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "GTE",
		"attr": "score",
		"val":  10,
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preds[0], ")::numeric") {
		t.Fatalf("expected numeric cast: %s", preds[0])
	}
}

func TestVRFilterSQL_RollupGT(t *testing.T) {
	cols := []shared.ColumnMeta{{Name: "itemCount", TypeId: "rollup"}}
	argN := 3
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "GT",
		"attr": "itemCount",
		"val":  1,
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("preds: %v", preds)
	}
	sql := preds[0]
	if !strings.Contains(sql, ")::numeric") || !strings.Contains(sql, " > $3::numeric") {
		t.Fatalf("sql: %s", sql)
	}
	if strings.Contains(sql, `"(`) {
		t.Fatalf("CASE expr quoted as identifier: %s", sql)
	}
	if len(args) != 1 {
		t.Fatalf("args: %v", args)
	}
}

func TestVRFilterSQL_NameEQNotQuoted(t *testing.T) {
	cols := []shared.ColumnMeta{{Name: "name", PgType: "text"}}
	argN := 3
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"type": "EQ",
		"attr": "name",
		"val":  "BBB",
	}, cols, &argN, &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("preds: %v", preds)
	}
	sql := preds[0]
	if strings.Contains(sql, `"(`) || strings.Contains(sql, `"(CASE`) {
		t.Fatalf("CASE expr quoted as identifier: %s", sql)
	}
	if !strings.Contains(sql, "data->>'name'") || !strings.Contains(sql, " = $3") {
		t.Fatalf("sql: %s", sql)
	}
}

func TestColumnPgTypeHonorsArrayConfig(t *testing.T) {
	// exercised via EffectivePgType path used by catalog; keep VR expr test local
	expr := vrArrayExpr("tags", "text[]")
	if !strings.Contains(expr, "text[]") || !strings.Contains(expr, "'tags'") {
		t.Fatalf("expr: %s", expr)
	}
}
