package migrator

import "testing"

func TestNamespaceFromDSN(t *testing.T) {
	tests := []struct {
		dsn  string
		want string
	}{
		{"postgresql://u:p@localhost:5432/lowcode?search_path=meta", "meta"},
		{"postgres://u:p@localhost:5432/lowcode?sslmode=disable&search_path=faas", "faas"},
		{"postgresql://u:p@localhost:5432/lowcode?search_path=meta,public", "meta"},
		{"postgresql://u:p@localhost:5432/lowcode", ""},
		{"postgresql://u:p@localhost:5432/lowcode?search_path=public", "public"},
		{"not a url", ""},
	}
	for _, tt := range tests {
		if got := NamespaceFromDSN(tt.dsn); got != tt.want {
			t.Errorf("NamespaceFromDSN(%q)=%q want %q", tt.dsn, got, tt.want)
		}
	}
}
