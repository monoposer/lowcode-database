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

func TestCompileRollupSubqueryRef(t *testing.T) {
	rollupSQL := `(
		SELECT COUNT(*) FROM public.orders AS _r
		WHERE _r.vendor_id = _b.id
	)`
	sql, err := Compile("CONCAT({{goods_count}}, '+1')", "_b", map[string]string{
		"goods_count": rollupSQL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "COUNT(*)") || !strings.Contains(sql, "'+1'") {
		t.Fatalf("expected rollup subquery in SQL, got %q", sql)
	}
	if strings.Contains(sql, "__lc_goods_count__") {
		t.Fatalf("placeholder must be replaced, got %q", sql)
	}
}

func TestCompileFormulaRefUsesStub(t *testing.T) {
	sql, err := Compile("{{base}} + {{other}}", "_b", map[string]string{
		"base":  "score",
		"other": StubRef("other"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "_b.score") {
		t.Fatalf("got %q", sql)
	}
}

func TestCompileLookupQualifiedRef(t *testing.T) {
	sql, err := Compile("CONCAT({{username}}, '-x')", "_b", map[string]string{
		"username": "lk_user.username",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "lk_user.username") {
		t.Fatalf("got %q", sql)
	}
}
