package logger

import (
	"strings"
	"testing"
)

func TestFormatSQLArgs(t *testing.T) {
	got := FormatSQLArgs([]any{"tok", int64(42), true})
	if got == "[]" || got == "" {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, `$1="tok"`) {
		t.Fatalf("got %q", got)
	}
}
