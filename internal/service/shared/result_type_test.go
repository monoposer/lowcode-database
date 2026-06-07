package shared

import "testing"

func TestRollupResultTypeId(t *testing.T) {
	if RollupResultTypeId("count", "text") != "number" {
		t.Fatal("count")
	}
	if RollupResultTypeId("max", "datetime") != "datetime" {
		t.Fatal("max datetime")
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
	if InferFormulaResultTypeId(`=TRIM({{s}})`) != "text" {
		t.Fatal("trim formula")
	}
	if InferFormulaResultTypeId(`=ISBLANK({{s}})`) != "boolean" {
		t.Fatal("isblank formula")
	}
	if InferFormulaResultTypeId(`=MIN({{a}},{{b}})`) != "number" {
		t.Fatal("min formula")
	}
	if InferFormulaResultTypeId(`=NOW()`) != "datetime" {
		t.Fatal("now formula")
	}
	if InferFormulaResultTypeId(`=WEEKDAY({{d}})`) != "number" {
		t.Fatal("weekday formula")
	}
	if InferFormulaResultTypeId(`=DATEVALUE({{s}})`) != "datetime" {
		t.Fatal("datevalue formula")
	}
}
