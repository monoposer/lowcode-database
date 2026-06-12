package authz

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileAuthorizer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authz.json")
	if err := os.WriteFile(path, []byte(`{
		"roles": {
			"admin": ["*:*"],
			"viewer": ["*:select", "schema:read"],
			"editor": ["orders:select", "orders:insert"]
		}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := NewFileAuthorizer(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	cases := []struct {
		name   string
		sub    Subject
		action Action
		res    Resource
		want   bool
	}{
		{
			name:   "admin all",
			sub:    Subject{Sub: "u1", Roles: []string{"admin"}},
			action: ActionDelete,
			res:    Resource{Type: "db", Table: "any"},
			want:   true,
		},
		{
			name:   "viewer select",
			sub:    Subject{Sub: "u1", Roles: []string{"viewer"}},
			action: ActionSelect,
			res:    Resource{Type: "db", Table: "orders"},
			want:   true,
		},
		{
			name:   "viewer insert denied",
			sub:    Subject{Sub: "u1", Roles: []string{"viewer"}},
			action: ActionInsert,
			res:    Resource{Type: "db", Table: "orders"},
			want:   false,
		},
		{
			name:   "editor table scoped",
			sub:    Subject{Sub: "u1", Roles: []string{"editor"}},
			action: ActionInsert,
			res:    Resource{Type: "db", Table: "orders"},
			want:   true,
		},
		{
			name:   "editor other table denied",
			sub:    Subject{Sub: "u1", Roles: []string{"editor"}},
			action: ActionInsert,
			res:    Resource{Type: "db", Table: "products"},
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.Allow(ctx, Request{User: tc.sub, Action: tc.action, Resource: tc.res})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("allow=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestMatchPermission(t *testing.T) {
	if !matchPermission("*:*", "orders", "select") {
		t.Fatal("*:* should match")
	}
	if !matchPermission("*:select", "orders", "select") {
		t.Fatal("*:select should match")
	}
	if matchPermission("orders:insert", "orders", "select") {
		t.Fatal("orders:insert should not match select")
	}
}
