package catalog_test

import (
	"context"
	"testing"

	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func TestColumnPgTypeSQL_BuiltinIgnoresColumnConfigArray(t *testing.T) {
	c := catalog.New(&shared.Base{})
	got := c.ColumnPgTypeSQL(context.Background(), "t", "text", map[string]any{"array": true})
	if got != "text" {
		t.Fatalf("got %q want text (column config.array must be ignored)", got)
	}
	got = c.ColumnPgTypeSQL(context.Background(), "t", "number", map[string]any{"array": true})
	if got != "numeric" {
		t.Fatalf("got %q want numeric", got)
	}
}

func TestEffectiveColumnTypePgType_MultiSelect(t *testing.T) {
	got, err := columntype.EffectiveColumnTypePgType(columntype.ColumnTypeSpec{PgType: "text", Array: true})
	if err != nil || got != "text[]" {
		t.Fatalf("got %q err=%v want text[]", got, err)
	}
	got, err = columntype.EffectiveColumnTypePgType(columntype.ColumnTypeSpec{PgType: "text"})
	if err != nil || got != "text" {
		t.Fatalf("got %q err=%v want text", got, err)
	}
}
