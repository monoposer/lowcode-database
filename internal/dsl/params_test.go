package dsl

import "testing"

func TestParamNames(t *testing.T) {
	filter := map[string]any{
		"type": "EQ",
		"attr": "username",
		"val":  "{username}",
	}
	got := ParamNames(filter)
	if len(got) != 1 || got[0] != "username" {
		t.Fatalf("ParamNames: %v", got)
	}
}

func TestSubstituteParams(t *testing.T) {
	filter := map[string]any{
		"type": "EQ",
		"attr": "username",
		"val":  "{username}",
	}
	out, err := SubstituteParams(filter, map[string]any{"username": "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if out["val"] != "alice" {
		t.Fatalf("val: %v", out["val"])
	}
}

func TestSubstituteParamsMissing(t *testing.T) {
	filter := map[string]any{
		"type": "EQ",
		"attr": "username",
		"val":  "{username}",
	}
	_, err := SubstituteParams(filter, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
