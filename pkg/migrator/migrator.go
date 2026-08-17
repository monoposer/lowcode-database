package migrator

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var fileRe = regexp.MustCompile(`^(\d+)_(.+)\.up\.sql$`)
var extRe = regexp.MustCompile(`(?i)CREATE\s+EXTENSION\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"([^"]+)"|'([^']+)'|([a-zA-Z0-9_]+))`)

type Result struct {
	Applied    []string `json:"applied,omitempty"`
	MissingExt []string `json:"missingExtensions,omitempty"`
}

// Apply runs *.up.sql files from fsys against databaseURL in filename order.
// SQL is expected to be idempotent (IF NOT EXISTS). No version table is written.
func Apply(ctx context.Context, databaseURL string, fsys fs.FS) error {
	_, err := ApplyResult(ctx, databaseURL, fsys, nil)
	return err
}

// ApplyWithConfig is Apply plus optional session GUCs (`SET` for the connection).
func ApplyWithConfig(ctx context.Context, databaseURL string, fsys fs.FS, sessionConfig map[string]string) error {
	_, err := ApplyResult(ctx, databaseURL, fsys, sessionConfig)
	return err
}

// ApplyResult is Apply plus the list of files executed. Fails before DDL if
// required extensions are not available on the server (e.g. PostGIS missing).
func ApplyResult(ctx context.Context, databaseURL string, fsys fs.FS, sessionConfig map[string]string) (Result, error) {
	var out Result
	if databaseURL == "" {
		return out, fmt.Errorf("database URL is required")
	}
	if fsys == nil {
		return out, fmt.Errorf("migrations filesystem is required")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return out, fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	files, err := listMigrationFiles(fsys)
	if err != nil {
		return out, err
	}

	bodies := make([][]byte, len(files))
	var allSQL strings.Builder
	for i, f := range files {
		body, err := fs.ReadFile(fsys, f.name)
		if err != nil {
			return out, fmt.Errorf("read %s: %w", f.name, err)
		}
		bodies[i] = body
		allSQL.Write(body)
		allSQL.WriteByte('\n')
	}
	missing, err := missingExtensions(ctx, pool, RequiredExtensions(allSQL.String()))
	if err != nil {
		return out, err
	}
	if len(missing) > 0 {
		out.MissingExt = missing
		return out, fmt.Errorf("postgres extensions not available: %s", strings.Join(missing, ", "))
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return out, fmt.Errorf("acquire: %w", err)
	}
	defer conn.Release()

	for k, v := range sessionConfig {
		if k == "" {
			continue
		}
		if _, err := conn.Exec(ctx, `SELECT set_config($1, $2, false)`, k, v); err != nil {
			return out, fmt.Errorf("set_config %s: %w", k, err)
		}
	}

	for i, f := range files {
		fmt.Printf("applying %s\n", path.Base(f.name))
		if _, err := conn.Exec(ctx, string(bodies[i])); err != nil {
			return out, fmt.Errorf("apply %s: %w", f.name, err)
		}
		out.Applied = append(out.Applied, f.name)
	}
	return out, nil
}

// RequiredExtensions extracts CREATE EXTENSION names from SQL.
func RequiredExtensions(sql string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range extRe.FindAllStringSubmatch(sql, -1) {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if name == "" {
			name = m[3]
		}
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func missingExtensions(ctx context.Context, pool *pgxpool.Pool, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	rows, err := pool.Query(ctx, `
		SELECT name FROM pg_available_extensions WHERE name = ANY($1)
	`, names)
	if err != nil {
		return nil, fmt.Errorf("check extensions: %w", err)
	}
	defer rows.Close()
	avail := map[string]struct{}{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		avail[n] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var missing []string
	for _, n := range names {
		if _, ok := avail[n]; !ok {
			missing = append(missing, n)
		}
	}
	return missing, nil
}

type migFile struct {
	version int
	name    string
}

func listMigrationFiles(fsys fs.FS) ([]migFile, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var files []migFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		m := fileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, _ := strconv.Atoi(m[1])
		files = append(files, migFile{version: v, name: e.Name()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}
