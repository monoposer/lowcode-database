package typespec_test

import (
	"strings"
	"testing"

	"github.com/monoposer/lowcode-database/pkg/typespec"
)

func TestCreateColumnTypeSQL_positivePrice(t *testing.T) {
	p, s := 18, 2
	sql, err := typespec.CreateColumnTypeSQL("tenant_acme", "positive_price", typespec.ColumnTypeSpec{
		PgType:    "numeric",
		Precision: &p,
		Scale:     &s,
		Checks:    []typespec.CheckSpec{{Expr: "VALUE > 0"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `CREATE DOMAIN "tenant_acme"."positive_price" AS numeric(18,2) CHECK (VALUE > 0)`
	if sql != want {
		t.Fatalf("got %q want %q", sql, want)
	}
}

func TestCreateColumnTypeSQL_textArray(t *testing.T) {
	sql, err := typespec.CreateColumnTypeSQL("tenant_a", "tag_list", typespec.ColumnTypeSpec{
		PgType:  "text[]",
		NotNull: true,
		Checks: []typespec.CheckSpec{{
			Expr: "array_length(VALUE, 1) > 0",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `AS text[]`) {
		t.Fatalf("expected text[] pgType: %s", sql)
	}
	if !strings.Contains(sql, "NOT NULL") {
		t.Fatalf("expected NOT NULL: %s", sql)
	}
}

func TestValidateColumnType_rejectsBuiltinName(t *testing.T) {
	err := typespec.ValidateColumnType(&typespec.ColumnType{
		Metadata: typespec.ColumnTypeMetadata{Name: "text"},
		Spec:     typespec.ColumnTypeSpec{PgType: "numeric"},
	})
	if err == nil {
		t.Fatal("expected conflict with builtin")
	}
}

func TestCanonicalPgTypes(t *testing.T) {
	want := []string{"text", "number", "datetime", "boolean", "jsonb", "point", "formula", "link", "lookup", "rollup"}
	got := typespec.ListPgTypes()
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("ListPgTypes()[%d]=%q want %q", i, got[i].ID, id)
		}
	}
	if typespec.CanonicalID("int8") != "number" || typespec.CanonicalID("timestamptz") != "datetime" {
		t.Fatal("canonical aliases")
	}
	if !typespec.AllowsArray("text") || typespec.AllowsArray("link") {
		t.Fatal("array modifier")
	}
	pt, ok := typespec.GetPgType("text_array")
	if !ok || pt.PgType != "text[]" || typespec.CanonicalID("text_array") != "text" {
		t.Fatalf("text_array: %+v ok=%v", pt, ok)
	}
}

func TestValidateTypeCatalog(t *testing.T) {
	c := typespec.TypeCatalog{
		APIVersion: typespec.APIVersion,
		Kind:       typespec.KindTypeCatalog,
		ColumnTypes: []typespec.ColumnType{{
			Metadata: typespec.ColumnTypeMetadata{Name: "email_addr"},
			Spec: typespec.ColumnTypeSpec{
				PgType: "text",
				Checks: []typespec.CheckSpec{{Expr: `VALUE ~ '@'`}},
			},
		}},
	}
	if err := typespec.ValidateTypeCatalog(&c); err != nil {
		t.Fatal(err)
	}
}
