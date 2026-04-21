package service

import (
	"testing"
)

func TestCompileFormulaExpression(t *testing.T) {
	sql, err := compileFormulaExpression("{{amount}} * 2 + {{tax}}", "_b", map[string]string{
		"amount": "c_amount",
		"tax":    "c_tax",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "_b.c_amount * 2 + _b.c_tax"
	if sql != want {
		t.Fatalf("got %q want %q", sql, want)
	}
}

func TestCompileFormulaUnknownColumn(t *testing.T) {
	_, err := compileFormulaExpression("{{missing}}", "_b", map[string]string{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIsVirtualKind(t *testing.T) {
	if !isVirtualKind("formula") || isVirtualKind("text") {
		t.Fatal("virtual kind check failed")
	}
}
