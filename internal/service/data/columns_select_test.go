package data

import (
	"testing"

	"github.com/solat/lowcode-database/internal/service/shared"
)

func TestColumnAllowSet(t *testing.T) {
	allow := columnAllowSet([]string{"name"})
	if !columnAllowed("name", allow) || columnAllowed("other", allow) {
		t.Fatal("allow set mismatch")
	}
}

func TestQueryableColumnNames(t *testing.T) {
	names := queryableColumnNames([]shared.FullColumnMeta{
		{Name: "id", Kind: "text"},
		{Name: "rel", Kind: "relationship"},
		{Name: "total", Kind: "formula"},
	})
	if len(names) != 2 || names[0] != "id" || names[1] != "total" {
		t.Fatalf("got %v", names)
	}
}
