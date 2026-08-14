package data

import (
	"strings"
	"testing"

	"github.com/monoposer/lowcode-database/internal/service/shared"
)

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

func TestVRFilterSQL_LegacyEQ(t *testing.T) {
	cols := []shared.ColumnMeta{{Name: "title", PgType: "text"}}
	argN := 1
	var args []any
	preds, err := vrFilterSQL(map[string]any{
		"field": "title",
		"op":    "eq",
		"value": "x",
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

func TestColumnPgTypeHonorsArrayConfig(t *testing.T) {
	// exercised via EffectivePgType path used by catalog; keep VR expr test local
	expr := vrArrayExpr("tags", "text[]")
	if !strings.Contains(expr, "text[]") || !strings.Contains(expr, "'tags'") {
		t.Fatalf("expr: %s", expr)
	}
}
