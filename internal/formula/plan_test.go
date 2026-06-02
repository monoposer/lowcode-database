package formula

import (
	"strings"
	"testing"
)

func TestBuildStepsChainUsesLateral(t *testing.T) {
	base := map[string]string{"score": "score"}
	defs := []Def{
		{Name: "double_score", Expr: "{{score}} * 2"},
		{Name: "with_bonus", Expr: "{{double_score}} + 10"},
	}
	steps, err := BuildSteps("_b", base, defs)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("steps: %d", len(steps))
	}
	if steps[0].Name != "double_score" || steps[0].Inline {
		t.Fatalf("referenced formula must use lateral: %+v", steps[0])
	}
	if steps[1].Name != "with_bonus" || steps[1].Inline {
		t.Fatalf("dependent must use lateral: %+v", steps[1])
	}
	if !strings.Contains(steps[1].SQL, `"_f_double_score"`) && !strings.Contains(steps[1].SQL, "_f_double_score") {
		t.Fatalf("with_bonus should reference lateral alias, got %q", steps[1].SQL)
	}
	if steps[0].LateralJoinSQL() == "" {
		t.Fatal("expected lateral join for double_score")
	}
}

func TestBuildStepsStandaloneInline(t *testing.T) {
	steps, err := BuildSteps("_b", map[string]string{"score": "score"}, []Def{
		{Name: "only", Expr: "{{score}} * 2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || !steps[0].Inline || steps[0].LateralJoinSQL() != "" {
		t.Fatalf("standalone formula should be inline: %+v", steps[0])
	}
}

func TestBuildStepsReusesRollup(t *testing.T) {
	rollupSQL := `(
		SELECT COUNT(*) FROM public.orders AS _r
		WHERE _r.vendor_id = _b.id
	)`
	steps, err := BuildSteps("_b", map[string]string{
		"goods_count": rollupSQL,
	}, []Def{
		{Name: "label", Expr: "CONCAT({{goods_count}}, '+1')"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || !steps[0].Inline {
		t.Fatalf("got %+v", steps)
	}
	if !strings.Contains(steps[0].SQL, "COUNT(*)") {
		t.Fatalf("rollup should be embedded: %q", steps[0].SQL)
	}
}
