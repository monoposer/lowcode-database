package shared

import "testing"

func TestRollupResultTypeId(t *testing.T) {
	if RollupResultTypeId("count", "text") != "number" {
		t.Fatal("count")
	}
	if RollupResultTypeId("max", "timestamptz") != "timestamptz" {
		t.Fatal("max timestamptz")
	}
	if RollupResultTypeId("sum", "int8") != "number" {
		t.Fatal("sum int8")
	}
}

func TestInferFormulaResultTypeId(t *testing.T) {
	if InferFormulaResultTypeId("={{qty}}*2") != "number" {
		t.Fatal("numeric formula")
	}
	if InferFormulaResultTypeId(`=CONCAT({{a}},"x")`) != "text" {
		t.Fatal("text formula")
	}
}
