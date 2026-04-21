package columntype_test

import (
	"testing"

	"github.com/solat/lowcode-database/internal/columntype"
)

func TestResolveBuiltInTypes(t *testing.T) {
	for _, id := range []string{"text", "int8", "formula", "enum", "relation_fk"} {
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

func TestListNonEmpty(t *testing.T) {
	if len(columntype.List()) < 10 {
		t.Fatalf("expected many built-in types, got %d", len(columntype.List()))
	}
}
