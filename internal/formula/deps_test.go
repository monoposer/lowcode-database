package formula

import (
	"errors"
	"testing"
)

func TestRefsDedupes(t *testing.T) {
	got := Refs("{{a}} + {{b}} + {{a}}")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("Refs: %v", got)
	}
}

func TestSortDependencyOrder(t *testing.T) {
	defs := []Def{
		{Name: "total", Expr: "{{base}} + {{bonus}}"},
		{Name: "bonus", Expr: "{{base}} * 0.1"},
		{Name: "base", Expr: "{{score}} * 2"},
	}
	ordered, err := Sort(defs)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{ordered[0].Name, ordered[1].Name, ordered[2].Name}
	if names[0] != "base" || names[1] != "bonus" || names[2] != "total" {
		t.Fatalf("order: %v", names)
	}
}

func TestSortSelfReference(t *testing.T) {
	_, err := Sort([]Def{{Name: "x", Expr: "{{x}} + 1"}})
	if err == nil {
		t.Fatal("expected self-reference error")
	}
}

func TestSortCycle(t *testing.T) {
	_, err := Sort([]Def{
		{Name: "a", Expr: "{{b}}"},
		{Name: "b", Expr: "{{a}}"},
	})
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("expected ErrCycle, got %v", err)
	}
}

func TestDetectCycleOnUpdate(t *testing.T) {
	existing := map[string]string{
		"a": "{{b}}",
		"b": "{{score}} * 2",
	}
	if err := DetectCycle(existing, "a", "{{b}}"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := DetectCycle(existing, "b", "{{a}}"); !errors.Is(err, ErrCycle) {
		t.Fatalf("expected cycle, got %v", err)
	}
}
