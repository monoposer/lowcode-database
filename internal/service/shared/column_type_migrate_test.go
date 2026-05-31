package shared

import (
	"strings"
	"testing"
)

func TestColumnAlterTypeUsing_textToBigint(t *testing.T) {
	u := ColumnAlterTypeUsing("text", "bigint", "c_add")
	if !strings.Contains(u, "~ '^-?[0-9]+$'") {
		t.Fatalf("expected integer pattern, got %q", u)
	}
	if !strings.Contains(u, "ELSE NULL END") {
		t.Fatalf("expected invalid values -> NULL, got %q", u)
	}
}

func TestColumnAlterTypeUsing_textToBoolean(t *testing.T) {
	u := ColumnAlterTypeUsing("text", "boolean", "c_flag")
	if !strings.Contains(u, "'true'") || !strings.Contains(u, "ELSE NULL END") {
		t.Fatalf("got %q", u)
	}
}

func TestColumnAlterTypeUsing_numericToText(t *testing.T) {
	u := ColumnAlterTypeUsing("bigint", "text", "c_n")
	if u != `"c_n"::text` {
		t.Fatalf("got %q", u)
	}
}
