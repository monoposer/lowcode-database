package migrator

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// NamespaceFromDSN returns the first search_path item from a postgres URL.
// Empty means the server default (typically public). SQL must not hardcode a schema.
func NamespaceFromDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return ""
	}
	return NormalizeNamespace(u.Query().Get("search_path"))
}

// NormalizeNamespace takes a search_path value (possibly "meta, public") and
// returns a single identifier, or empty if unset / invalid.
func NormalizeNamespace(searchPath string) string {
	sp := strings.TrimSpace(searchPath)
	if sp == "" {
		return ""
	}
	first := strings.Split(sp, ",")[0]
	first = strings.Trim(strings.TrimSpace(first), `"'`)
	if first == "" || !identRe.MatchString(first) {
		return ""
	}
	return first
}

// EnsureNamespace creates the schema (unless public) and sets search_path on conn.
func EnsureNamespace(ctx context.Context, conn *pgxpool.Conn, name string) error {
	name = NormalizeNamespace(name)
	if name == "" {
		return nil
	}
	ident := pgx.Identifier{name}.Sanitize()
	if name != "public" {
		if _, err := conn.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+ident); err != nil {
			return fmt.Errorf("create schema %s: %w", name, err)
		}
	}
	if _, err := conn.Exec(ctx, "SET search_path TO "+ident); err != nil {
		return fmt.Errorf("set search_path %s: %w", name, err)
	}
	return nil
}
