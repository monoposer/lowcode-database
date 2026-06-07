package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/monoposer/lowcode-database/internal/config"
	"github.com/monoposer/lowcode-database/internal/migrator"
	"github.com/monoposer/lowcode-database/migrations"
)

func main() {
	var (
		target = flag.String("target", "all", "migration target: meta | data | all")
		dir    = flag.String("dir", "", "migrations directory of *.up.sql (default: embedded migrations/{target})")
		dsn    = flag.String("database-url", "", "postgres URL (overrides env)")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	targets, err := expandTargets(*target)
	if err != nil {
		log.Fatal(err)
	}
	if *dir != "" && len(targets) > 1 {
		log.Fatal("-dir requires a single -target (meta or data)")
	}
	if *dsn != "" && len(targets) > 1 {
		log.Fatal("-database-url requires a single -target (meta or data)")
	}

	var cfg *config.Config
	if *dsn == "" {
		cfg, err = config.Load()
		if err != nil {
			log.Fatalf("config: %v", err)
		}
	}

	for _, t := range targets {
		fsys, src, err := openMigrations(t, *dir)
		if err != nil {
			log.Fatalf("migrations: %v", err)
		}
		urls, err := resolveDSNs(ctx, t, *dsn, cfg)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("migrations: %s\n", src)
		for _, dbURL := range urls {
			fmt.Printf("migrating %s database: %s\n", t, redactDSN(dbURL))
			if err := migrator.Apply(ctx, dbURL, fsys); err != nil {
				log.Fatalf("migrate %s (%s): %v", t, redactDSN(dbURL), err)
			}
		}
	}
	fmt.Println("done")
}

func expandTargets(target string) ([]string, error) {
	switch target {
	case "all":
		return []string{"meta", "data"}, nil
	case "meta", "data":
		return []string{target}, nil
	default:
		return nil, fmt.Errorf("unknown target %q (use meta, data, or all)", target)
	}
}

func openMigrations(target, dir string) (fs.FS, string, error) {
	if dir != "" {
		return os.DirFS(dir), dir, nil
	}
	fsys, err := migrations.FS(target)
	if err != nil {
		return nil, "", err
	}
	return fsys, "embed:migrations/" + target, nil
}

func resolveDSN(target, dsn string, cfg *config.Config) (string, error) {
	urls, err := resolveDSNs(context.Background(), target, dsn, cfg)
	if err != nil {
		return "", err
	}
	if len(urls) == 0 {
		return "", fmt.Errorf("no database URL")
	}
	return urls[0], nil
}

func resolveDSNs(ctx context.Context, target, dsn string, cfg *config.Config) ([]string, error) {
	if dsn != "" {
		return []string{dsn}, nil
	}
	switch target {
	case "meta":
		if cfg == nil || cfg.MetaDatabaseURL == "" {
			return nil, fmt.Errorf("META_DATABASE_URL is required")
		}
		return []string{cfg.MetaDatabaseURL}, nil
	case "data":
		urls, err := listTenantDataDSNs(ctx, cfg)
		if err != nil {
			return nil, err
		}
		if len(urls) == 0 {
			fallback := firstNonEmpty(os.Getenv("DATA_DATABASE_URL"), "")
			if cfg != nil {
				fallback = firstNonEmpty(fallback, cfg.DefaultTenantDataDSN)
			}
			if fallback == "" {
				return nil, fmt.Errorf("database URL required: pass -database-url or set DEFAULT_TENANT_DATA_DSN")
			}
			return []string{fallback}, nil
		}
		return urls, nil
	default:
		return nil, fmt.Errorf("unknown target %q", target)
	}
}

func listTenantDataDSNs(ctx context.Context, cfg *config.Config) ([]string, error) {
	if cfg == nil || cfg.MetaDatabaseURL == "" {
		return nil, nil
	}
	pool, err := pgxpool.New(ctx, cfg.MetaDatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("meta connect: %w", err)
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT COALESCE(NULLIF(data_dsn_write, ''), data_dsn)
		FROM tenants
		WHERE status = 'active' AND COALESCE(NULLIF(data_dsn_write, ''), data_dsn) <> ''
	`)
	if err != nil {
		// Older meta without replica columns: fall back to data_dsn only.
		rows, err = pool.Query(ctx, `
			SELECT DISTINCT data_dsn FROM tenants WHERE status = 'active' AND data_dsn <> ''
		`)
		if err != nil {
			return nil, nil
		}
	}
	defer rows.Close()
	seen := map[string]struct{}{}
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	if cfg.DefaultTenantDataDSN != "" {
		if _, ok := seen[cfg.DefaultTenantDataDSN]; !ok {
			out = append(out, cfg.DefaultTenantDataDSN)
		}
	}
	return out, rows.Err()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func redactDSN(dsn string) string {
	// hide password in logs: postgresql://user:pass@host/db -> postgresql://user:***@host/db
	at := len("postgresql://")
	if len(dsn) <= at {
		return dsn
	}
	rest := dsn[at:]
	for i := 0; i < len(rest); i++ {
		if rest[i] == '@' {
			userPart := rest[:i]
			if colon := len(userPart); colon > 0 {
				for j := 0; j < len(userPart); j++ {
					if userPart[j] == ':' {
						return dsn[:at] + userPart[:j+1] + "***" + rest[i:]
					}
				}
			}
			break
		}
	}
	return dsn
}
