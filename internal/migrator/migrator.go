package migrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var fileRe = regexp.MustCompile(`^(\d+)_(.+)\.up\.sql$`)

// Apply runs pending *.up.sql migrations from dir against databaseURL.
func Apply(ctx context.Context, databaseURL, dir string) error {
	if databaseURL == "" {
		return fmt.Errorf("database URL is required")
	}
	if dir == "" {
		return fmt.Errorf("migrations directory is required")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return err
	}

	current, err := currentVersion(ctx, pool)
	if err != nil {
		return err
	}

	files, err := listMigrationFiles(dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.version <= current {
			continue
		}
		body, err := os.ReadFile(f.path)
		if err != nil {
			return fmt.Errorf("read %s: %w", f.path, err)
		}
		fmt.Printf("applying %s\n", filepath.Base(f.path))
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", f.path, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO lc_schema_migrations (version, name) VALUES ($1, $2)`,
			f.version, f.name,
		); err != nil {
			return fmt.Errorf("record migration %d: %w", f.version, err)
		}
	}

	return nil
}

type migFile struct {
	version int
	name    string
	path    string
}

func listMigrationFiles(dir string) ([]migFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
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
		files = append(files, migFile{
			version: v,
			name:    m[2],
			path:    filepath.Join(dir, e.Name()),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS lc_schema_migrations (
			version    INT PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

func currentVersion(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var v int
	err := pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM lc_schema_migrations`).Scan(&v)
	return v, err
}

// DefaultDir returns docker/postgres/migrations/{target} relative to project root.
func DefaultDir(target string) (string, error) {
	root, err := findProjectRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "docker", "postgres", "migrations", target), nil
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "docker", "postgres", "migrations")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("project root not found (go.mod + docker/postgres/migrations)")
		}
		dir = parent
	}
}
