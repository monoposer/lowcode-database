package columntype_test

import (
	"testing"

	"github.com/solat/lowcode-database/internal/columntype"
)

func TestResolveBuiltInTypes(t *testing.T) {
	for _, id := range []string{"text", "int8", "formula", "relation_fk"} {
		if _, err := columntype.Resolve(id); err != nil {
			t.Fatalf("Resolve(%q): %v", id, err)
		}
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
	for _, id := range []string{"formula", "relationship", "lookup", "rollup"} {
		if got := columntype.PgType(id); got != "" {
			t.Fatalf("PgType(%q) = %q, want empty", id, got)
		}
	}
	if got := columntype.PgType("text"); got != "text" {
		t.Fatalf("PgType(text) = %q", got)
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

func TestListNonEmpty(t *testing.T) {
	if len(columntype.List()) < 10 {
		t.Fatalf("expected many built-in types, got %d", len(columntype.List()))
	}
}
