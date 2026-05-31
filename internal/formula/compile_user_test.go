package formula

import (
	"strings"
	"testing"
)

func TestCompileUserIFExpression(t *testing.T) {
	sql, err := Compile(`IF({{add}} = 'hi','h','f')`, "_b", map[string]string{"add": "c_add"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "_b.c_add") || !strings.Contains(sql, "CASE WHEN") {
		t.Fatalf("got %q", sql)
	}
}
