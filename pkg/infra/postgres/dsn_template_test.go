package postgres

import (
	"strings"
	"testing"
)

func TestApplyDataDSNTemplate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, tmpl, tenant, want string
	}{
		{"literal shared DB", "postgresql://u:p@host:5432/lowcode", "ws1", "postgresql://u:p@host:5432/lowcode"},
		{"per-tenant DB", "postgresql://u:p@host:5432/%s", "ws1", "postgresql://u:p@host:5432/ws1"},
		{"empty", "", "ws1", ""},
		{"whitespace literal", "  postgresql://u:p@host:5432/lowcode  ", "ws1", "postgresql://u:p@host:5432/lowcode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applyDataDSNTemplate(tc.tmpl, tc.tenant)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if strings.Contains(got, "%!(EXTRA") {
				t.Fatalf("corrupt sprintf DSN: %q", got)
			}
		})
	}
}
