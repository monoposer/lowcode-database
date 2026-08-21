package numeric

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestRoundHalfUp(t *testing.T) {
	spec := Spec{FinancialMode: true, Scale: 2, RoundingMode: RoundHalfUp}
	cases := []struct {
		in   string
		want string
	}{
		{"1.234", "1.23"},
		{"1.235", "1.24"},
		{"-1.235", "-1.24"},
	}
	for _, c := range cases {
		d, _ := decimal.NewFromString(c.in)
		got := Round(d, spec).StringFixed(2)
		if got != c.want {
			t.Fatalf("%s: got %s want %s", c.in, got, c.want)
		}
	}
}

func TestRoundCeilFloor(t *testing.T) {
	d, _ := decimal.NewFromString("1.231")
	if got := RoundToScale(d, 2, RoundCeil).StringFixed(2); got != "1.24" {
		t.Fatalf("ceil got %s", got)
	}
	if got := RoundToScale(d, 2, RoundFloor).StringFixed(2); got != "1.23" {
		t.Fatalf("floor got %s", got)
	}
	if got := RoundToScale(d, 2, RoundTruncate).StringFixed(2); got != "1.23" {
		t.Fatalf("truncate got %s", got)
	}
}

func TestDecimalArithmetic(t *testing.T) {
	a, _ := FromAny(0.1)
	b, _ := FromAny(0.2)
	sum := Add(a, b)
	if sum.String() != "0.3" {
		t.Fatalf("0.1+0.2 = %s", sum)
	}
	prod := Mul(decimal.NewFromInt(3), decimal.NewFromInt(7))
	if prod.IntPart() != 21 {
		t.Fatalf("3*7 = %s", prod)
	}
}

func TestNormalizeAny(t *testing.T) {
	spec := Spec{FinancialMode: true, Scale: 2, RoundingMode: RoundHalfUp}
	got, ok := NormalizeAny(10.005, spec)
	if !ok || got != 10.01 {
		t.Fatalf("got %v ok=%v", got, ok)
	}
}

func TestParseRoundingMode(t *testing.T) {
	for _, s := range []string{"half_up", "ceil", "floor", "truncate", "half_even"} {
		if _, err := ParseRoundingMode(s); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ParseRoundingMode("bogus"); err == nil {
		t.Fatal("expected error")
	}
}
