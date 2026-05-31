package formula

import (
	"strings"
	"testing"
)

func TestCompileSimpleArithmetic(t *testing.T) {
	sql, err := Compile("{{amount}} * 2 + {{tax}}", "_b", map[string]string{
		"amount": "c_amount",
		"tax":    "c_tax",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "_b.c_amount") || !strings.Contains(sql, "_b.c_tax") {
		t.Fatalf("expected qualified refs, got %q", sql)
	}
}

func TestCompileSumFunction(t *testing.T) {
	sql, err := Compile("SUM({{amount}}, {{tax}})", "_b", map[string]string{
		"amount": "c_amount",
		"tax":    "c_tax",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "_b.c_amount") || !strings.Contains(sql, "_b.c_tax") {
		t.Fatalf("got %q", sql)
	}
}

func TestCompileIfFunction(t *testing.T) {
	sql, err := Compile("IF({{qty}}>0, {{price}} * {{qty}}, 0)", "_b", map[string]string{
		"qty":   "c_qty",
		"price": "c_price",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "CASE WHEN") {
		t.Fatalf("expected CASE WHEN, got %q", sql)
	}
}

func TestCompileUnknownColumn(t *testing.T) {
	_, err := Compile("{{missing}}", "_b", map[string]string{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompileEmpty(t *testing.T) {
	sql, err := Compile("", "_b", nil)
	if err != nil || sql != "NULL" {
		t.Fatalf("got %q, %v", sql, err)
	}
}

func TestCompileLeadingEquals(t *testing.T) {
	sql, err := Compile("={{score}} * 2", "_b", map[string]string{"score": "c_score"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "_b.c_score") {
		t.Fatalf("got %q", sql)
	}
}
