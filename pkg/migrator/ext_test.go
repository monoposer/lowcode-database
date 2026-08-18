package migrator

import "testing"

func TestRequiredExtensions(t *testing.T) {
	sql := `
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";
`
	got := RequiredExtensions(sql)
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
	if got[0] != "pgcrypto" || got[1] != "pg_stat_statements" {
		t.Fatalf("got %#v", got)
	}
}
