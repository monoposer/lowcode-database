package schema

import (
	"errors"
	"testing"

	"github.com/solat/lowcode-database/internal/formula"
	"github.com/solat/lowcode-database/internal/service/shared"
)

func TestValidationFormulaRefsIncludesFormulaStub(t *testing.T) {
	cols := []shared.FullColumnMeta{
		{Name: "score", Kind: "int8", TypeId: "int8"},
		{Name: "base", Kind: "formula", TypeId: "formula", Config: map[string]any{"expression": "{{score}} * 2"}},
	}
	refs := validationFormulaRefs(cols, "total")
	if refs["score"] != "score" {
		t.Fatalf("score ref: %q", refs["score"])
	}
	if refs["base"] != formula.StubRef("base") {
		t.Fatalf("base stub: %q", refs["base"])
	}
	if _, ok := refs["total"]; ok {
		t.Fatal("editing column should not be in refs")
	}
}

func TestFormulaRefAllowedIncludesFormula(t *testing.T) {
	if !shared.FormulaRefAllowed("formula") {
		t.Fatal("formula columns should be referenceable")
	}
}

func TestDetectCycleWrapped(t *testing.T) {
	err := formula.DetectCycle(map[string]string{
		"a": "{{b}}",
	}, "b", "{{a}}")
	if !errors.Is(err, formula.ErrCycle) {
		t.Fatalf("got %v", err)
	}
}
