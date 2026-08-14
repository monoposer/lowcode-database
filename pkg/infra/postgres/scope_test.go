package postgres

import "testing"

func TestWhereOmitsEmpty(t *testing.T) {
	clause, args, next := Where(1, Eq{Col: "tenant_id", Val: "w1"}, Eq{Col: "base_id", Val: ""})
	if clause != "tenant_id = $1" || next != 2 || len(args) != 1 || args[0] != "w1" {
		t.Fatalf("got clause=%q args=%v next=%d", clause, args, next)
	}
}

func TestAndWhere(t *testing.T) {
	sql, args, next := AndWhere("SELECT 1 FROM t WHERE name = $1", []any{"n"}, 2, TenantBase("w", "b")...)
	want := "SELECT 1 FROM t WHERE name = $1 AND tenant_id = $2 AND base_id = $3"
	if sql != want || next != 4 || len(args) != 3 {
		t.Fatalf("got sql=%q args=%v next=%d", sql, args, next)
	}
}
