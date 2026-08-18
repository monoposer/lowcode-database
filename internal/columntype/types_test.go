package columntype_test

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/columntype"
)

func TestResolveBuiltInTypes(t *testing.T) {
	for _, id := range []string{"text", "number", "formula", "link"} {
		if _, err := columntype.Resolve(id); err != nil {
			t.Fatalf("Resolve(%q): %v", id, err)
		}
	}
	if _, err := columntype.Resolve("int8"); err == nil {
		t.Fatal("expected error for removed alias int8")
	}
	if _, err := columntype.Resolve("custom_foo"); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestIsVirtual(t *testing.T) {
	if !columntype.IsVirtual("formula") || columntype.IsVirtual("text") {
		t.Fatal("virtual detection wrong")
	}
}

func TestVirtualTypesHaveNoPgType(t *testing.T) {
	for _, id := range []string{"formula", "link", "lookup", "rollup"} {
		if got := columntype.PgType(id); got != "" {
			t.Fatalf("PgType(%q) = %q, want empty", id, got)
		}
	}
	if got := columntype.PgType("text"); got != "text" {
		t.Fatalf("PgType(text) = %q", got)
	}
	if got := columntype.PgType("number"); got != "numeric" {
		t.Fatalf("PgType(number) = %q", got)
	}
	if got := columntype.PgType("datetime"); got != "timestamptz" {
		t.Fatalf("PgType(datetime) = %q", got)
	}
}

func TestNoEnumBuiltInType(t *testing.T) {
	if columntype.IsBuiltIn("enum") {
		t.Fatal("enum should not be a built-in column type")
	}
	if _, err := columntype.Resolve("enum"); err == nil {
		t.Fatal("expected Resolve(enum) to fail")
	}
}

func TestNoPointBuiltInType(t *testing.T) {
	if columntype.IsBuiltIn("point") {
		t.Fatal("point should not be a built-in column type")
	}
	if _, err := columntype.Resolve("point"); err == nil {
		t.Fatal("expected Resolve(point) to fail")
	}
}

func TestListNonEmpty(t *testing.T) {
	if len(columntype.List()) != 9 {
		t.Fatalf("expected 9 built-in types, got %d", len(columntype.List()))
	}
}
