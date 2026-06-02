package data

import (
	"strings"
	"testing"
)

func TestJoinAliasRegistrySharedRel(t *testing.T) {
	r := newJoinAliasRegistry()
	a := r.sharedRelRowAlias("order")
	b := r.sharedRelRowAlias("order")
	if a != b {
		t.Fatalf("same relationship should share alias: %q vs %q", a, b)
	}
	c := r.sharedRelRowAlias("goods")
	if c == a {
		t.Fatalf("different relationships need different aliases")
	}
}

func TestAppendJoinDedupesAndSpaces(t *testing.T) {
	r := newJoinAliasRegistry()
	sql := `LEFT JOIN "public"."user" AS "u" ON "o"."user_id" = "u".id`
	got := r.appendJoin(sql)
	if !strings.HasPrefix(got, " ") {
		t.Fatalf("join fragment must start with space: %q", got)
	}
	if r.appendJoin(sql) != "" {
		t.Fatal("expected duplicate suppressed")
	}
}

func TestEnsureHopJoinReusesAlias(t *testing.T) {
	r := newJoinAliasRegistry()
	build := func(a string) string {
		return `LEFT JOIN "public"."user" AS "` + a + `" ON "o"."user_id" = "` + a + `".id`
	}
	a1, sql1 := r.ensureHopJoin("lk_rel_order", "public", "user", "username", build)
	a2, sql2 := r.ensureHopJoin("lk_rel_order", "public", "user", "username", build)
	if a1 != a2 {
		t.Fatalf("aliases: %q vs %q", a1, a2)
	}
	if sql1 == "" || sql2 != "" {
		t.Fatalf("sql1=%q sql2=%q", sql1, sql2)
	}
}
