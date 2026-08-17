package columntype_test

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/columntype"
)

func TestValidateColumnType_rejectsBuiltinName(t *testing.T) {
	err := columntype.ValidateColumnType(&columntype.ColumnType{
		Metadata: columntype.ColumnTypeMetadata{Name: "text"},
		Spec:     columntype.ColumnTypeSpec{PgType: "numeric"},
	})
	if err == nil {
		t.Fatal("expected conflict with builtin")
	}
}

func TestCanonicalPgTypes(t *testing.T) {
	want := []string{"text", "number", "datetime", "boolean", "jsonb", "point", "formula", "link", "lookup", "rollup"}
	got := columntype.ListPgTypes()
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("ListPgTypes()[%d]=%q want %q", i, got[i].ID, id)
		}
	}
	if _, ok := columntype.GetPgType("int8"); ok {
		t.Fatal("int8 alias should be removed")
	}
	text, ok := columntype.GetPgType("text")
	if !ok || len(text.Modifiers) != 0 {
		t.Fatalf("text should have no array modifier, mods=%v", text.Modifiers)
	}
}

func TestValidateColumnTypeSpec_Array(t *testing.T) {
	if err := columntype.ValidateColumnTypeSpec(&columntype.ColumnTypeSpec{PgType: "text", Array: true}); err != nil {
		t.Fatal(err)
	}
	if err := columntype.ValidateColumnTypeSpec(&columntype.ColumnTypeSpec{PgType: "datetime"}); err != nil {
		t.Fatal(err)
	}
	if err := columntype.ValidateColumnTypeSpec(&columntype.ColumnTypeSpec{PgType: "string", Array: true}); err == nil {
		t.Fatal("expected error for string (not allowed)")
	}
	if err := columntype.ValidateColumnTypeSpec(&columntype.ColumnTypeSpec{PgType: "link", Array: true}); err == nil {
		t.Fatal("expected error for disallowed pgType")
	}
	if err := columntype.ValidateColumnTypeSpec(&columntype.ColumnTypeSpec{PgType: "point"}); err == nil {
		t.Fatal("expected error for point")
	}
	got, err := columntype.EffectiveColumnTypePgType(columntype.ColumnTypeSpec{PgType: "text", Array: true})
	if err != nil || got != "text[]" {
		t.Fatalf("got %q err=%v", got, err)
	}
	got, err = columntype.EffectiveColumnTypePgType(columntype.ColumnTypeSpec{PgType: "datetime", Array: true})
	if err != nil || got != "timestamptz[]" {
		t.Fatalf("datetime array: got %q err=%v", got, err)
	}
}

func TestNormalizeColumnType_pgTypeTrim(t *testing.T) {
	got := columntype.NormalizeColumnType(columntype.ColumnType{
		Metadata: columntype.ColumnTypeMetadata{Name: "select"},
		Spec:     columntype.ColumnTypeSpec{PgType: " text "},
	})
	if got.Spec.PgType != "text" {
		t.Fatalf("pgType=%q want text", got.Spec.PgType)
	}
}

func TestValidateTypeCatalog(t *testing.T) {
	c := columntype.TypeCatalog{
		APIVersion: columntype.APIVersion,
		Kind:       columntype.KindTypeCatalog,
		ColumnTypes: []columntype.ColumnType{{
			Metadata: columntype.ColumnTypeMetadata{Name: "email_addr"},
			Spec: columntype.ColumnTypeSpec{
				PgType: "text",
				Checks: []columntype.CheckSpec{{Expr: `VALUE ~ '@'`}},
			},
		}},
	}
	if err := columntype.ValidateTypeCatalog(&c); err != nil {
		t.Fatal(err)
	}
}

func TestResolveUnderlyingPgType_numeric(t *testing.T) {
	p, s := 18, 2
	got, err := columntype.ResolveUnderlyingPgType("number", &p, &s)
	if err != nil {
		t.Fatal(err)
	}
	if got != "numeric(18,2)" {
		t.Fatalf("got %q", got)
	}
	got, err = columntype.ResolveUnderlyingPgType("text[]", nil, nil)
	if err != nil || got != "text[]" {
		t.Fatalf("text[]: %q %v", got, err)
	}
}
