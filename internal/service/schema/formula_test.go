package schema

import (
	"errors"
	"testing"

	"github.com/monoposer/lowcode-database/internal/formula"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func TestKnownFormulaColumnsOmitsEditing(t *testing.T) {
	cols := []shared.FullColumnMeta{
		{Name: "score", Kind: "int8", TypeId: "int8"},
		{Name: "base", Kind: "formula", TypeId: "formula", Config: map[string]any{"expression": "{{score}} * 2"}},
	}
	known := knownFormulaColumns(cols, "total")
	if _, ok := known["score"]; !ok {
		t.Fatal("score should be known")
	}
	if _, ok := known["base"]; !ok {
		t.Fatal("base formula should be known")
	}
	if _, ok := known["total"]; ok {
		t.Fatal("editing column should not be in known")
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
