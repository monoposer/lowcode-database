package formula

import (
	"testing"
	"time"
)

func TestEvalDecimalArithmetic(t *testing.T) {
	got, err := EvalExpr("0.1 + 0.2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 0.3 {
		t.Fatalf("0.1+0.2 = %#v", got)
	}
}

func TestEvalCeilFloorRound(t *testing.T) {
	got, err := EvalExpr("CEIL(1.231, 2)", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 1.24 {
		t.Fatalf("CEIL got %#v", got)
	}
	got, err = EvalExpr("FLOOR(1.239, 2)", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 1.23 {
		t.Fatalf("FLOOR got %#v", got)
	}
	got, err = EvalExpr("ROUND(1.235, 2)", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 1.24 {
		t.Fatalf("ROUND got %#v", got)
	}
}

func TestEvalSimpleArithmetic(t *testing.T) {
	got, err := EvalExpr("{{amount}} * 2 + {{tax}}", map[string]any{"amount": 10.0, "tax": 1.5})
	if err != nil {
		t.Fatal(err)
	}
	n, ok := got.(float64)
	if !ok || n != 21.5 {
		t.Fatalf("got %#v", got)
	}
}

func TestEvalSumFunction(t *testing.T) {
	got, err := EvalExpr("SUM({{amount}}, {{tax}})", map[string]any{"amount": 3.0, "tax": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 5 {
		t.Fatalf("got %#v", got)
	}
}

func TestEvalIfFunction(t *testing.T) {
	got, err := EvalExpr("IF({{qty}}>0, {{price}} * {{qty}}, 0)", map[string]any{"qty": 2.0, "price": 4.0})
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 8 {
		t.Fatalf("got %#v", got)
	}
}

func TestParseUnknownColumnStillParses(t *testing.T) {
	if err := Validate("{{missing}}", map[string]struct{}{}); err == nil {
		t.Fatal("expected unknown column")
	}
}

func TestEvalEmpty(t *testing.T) {
	got, err := EvalExpr("", nil)
	if err != nil || got != nil {
		t.Fatalf("got %#v %v", got, err)
	}
}

func TestEvalLeadingEquals(t *testing.T) {
	got, err := EvalExpr("={{score}} * 2", map[string]any{"score": 3.0})
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 6 {
		t.Fatalf("got %#v", got)
	}
}

func TestEvalConcat(t *testing.T) {
	got, err := EvalExpr(`CONCAT({{goods_count}}, "+1")`, map[string]any{"goods_count": 4.0})
	if err != nil {
		t.Fatal(err)
	}
	if got.(string) != "4+1" {
		t.Fatalf("got %#v", got)
	}
}

func TestEvalUserIFExpression(t *testing.T) {
	got, err := EvalExpr(`IF({{add}} = 'hi',"h","f")`, map[string]any{"add": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "h" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseEfpAST(t *testing.T) {
	n, err := Parse("=SUM({{a}}+{{b}}*2)/2")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Eval(n, map[string]any{"a": 1.0, "b": 3.0})
	if err != nil {
		t.Fatal(err)
	}
	// (1+3*2)/2 = 3.5
	if got.(float64) != 3.5 {
		t.Fatalf("got %#v", got)
	}
}

func TestEvalMinMaxInt(t *testing.T) {
	got, err := EvalExpr("MIN({{a}}, {{b}}, 0)", map[string]any{"a": 4.0, "b": -1.5})
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != -1.5 {
		t.Fatalf("MIN got %#v", got)
	}
	got, err = EvalExpr("MAX({{a}}, {{b}})", map[string]any{"a": 4.0, "b": -1.5})
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 4 {
		t.Fatalf("MAX got %#v", got)
	}
	got, err = EvalExpr("INT({{x}})", map[string]any{"x": -1.2})
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != -2 {
		t.Fatalf("INT got %#v", got)
	}
}

func TestEvalIsBlank(t *testing.T) {
	got, err := EvalExpr("ISBLANK({{name}})", map[string]any{"name": ""})
	if err != nil {
		t.Fatal(err)
	}
	if got != true {
		t.Fatalf("empty string got %#v", got)
	}
	got, err = EvalExpr("ISBLANK({{name}})", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if got != true {
		t.Fatalf("missing got %#v", got)
	}
	got, err = EvalExpr("ISBLANK({{name}})", map[string]any{"name": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if got != false {
		t.Fatalf("present got %#v", got)
	}
}

func TestEvalTrimUpperLower(t *testing.T) {
	got, err := EvalExpr(`TRIM({{s}})`, map[string]any{"s": "  a   b  "})
	if err != nil {
		t.Fatal(err)
	}
	if got != "a b" {
		t.Fatalf("TRIM got %#v", got)
	}
	got, err = EvalExpr(`UPPER({{s}})`, map[string]any{"s": "Ab"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "AB" {
		t.Fatalf("UPPER got %#v", got)
	}
	got, err = EvalExpr(`LOWER({{s}})`, map[string]any{"s": "Ab"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "ab" {
		t.Fatalf("LOWER got %#v", got)
	}
}

func TestEvalMIDAndIDCardDate(t *testing.T) {
	got, err := EvalExpr(`MID({{id_card}},7,8)`, map[string]any{"id_card": "110101199003071234"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "19900307" {
		t.Fatalf("MID got %#v", got)
	}
	got, err = EvalExpr(`DATE(MID({{id_card}},7,4), MID({{id_card}},11,2), MID({{id_card}},13,2))`, map[string]any{
		"id_card": "110101199003071234",
	})
	if err != nil {
		t.Fatal(err)
	}
	tm, ok := got.(time.Time)
	if !ok {
		t.Fatalf("DATE got %#v", got)
	}
	if y, m, d := tm.Date(); y != 1990 || m != time.March || d != 7 {
		t.Fatalf("DATE got %v", tm)
	}
}

func TestEvalNowDateValueWeekday(t *testing.T) {
	prev := nowFn
	fixed := time.Date(2026, 8, 14, 15, 4, 5, 0, time.Local)
	nowFn = func() time.Time { return fixed }
	defer func() { nowFn = prev }()

	got, err := EvalExpr("NOW()", nil)
	if err != nil {
		t.Fatal(err)
	}
	tm := got.(time.Time)
	if !tm.Equal(fixed) {
		t.Fatalf("NOW got %v", tm)
	}

	got, err = EvalExpr(`DATEVALUE({{s}})`, map[string]any{"s": "1990/3/7"})
	if err != nil {
		t.Fatal(err)
	}
	d := got.(time.Time)
	if y, m, day := d.Date(); y != 1990 || m != time.March || day != 7 {
		t.Fatalf("DATEVALUE got %v", d)
	}
	if d.Hour() != 0 {
		t.Fatalf("DATEVALUE should be date-only, got %v", d)
	}

	got, err = EvalExpr(`WEEKDAY(DATEVALUE("1990-03-07"))`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 4 { // Wednesday, return_type 1: Sun=1
		t.Fatalf("WEEKDAY type1 got %#v", got)
	}
	got, err = EvalExpr(`WEEKDAY(DATEVALUE("1990-03-07"), 2)`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.(float64) != 3 { // Mon=1
		t.Fatalf("WEEKDAY type2 got %#v", got)
	}
}
