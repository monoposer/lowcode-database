package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/monoposer/lowcode-database/migrations"
	"github.com/monoposer/lowcode-database/pkg/config"
	"github.com/monoposer/lowcode-database/pkg/migrator"
)

func main() {
	var (
		dir = flag.String("dir", "", "migrations directory of *.up.sql (default: embedded migrations/meta)")
		dsn = flag.String("database-url", "", "postgres URL (default: META_DATABASE_URL)")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	dbURL := *dsn
	if dbURL == "" {
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("config: %v", err)
		}
		dbURL = cfg.MetaDatabaseURL
	}

	fsys, src, err := openMigrations(*dir)
	if err != nil {
		log.Fatalf("migrations: %v", err)
	}
	fmt.Printf("migrations: %s\n", src)
	if ns := migrator.NamespaceFromDSN(dbURL); ns != "" {
		fmt.Printf("namespace: %s\n", ns)
	}
	fmt.Printf("migrating: %s\n", redactDSN(dbURL))
	if err := migrator.Apply(ctx, dbURL, fsys); err != nil {
		log.Fatalf("migrate meta (%s): %v", redactDSN(dbURL), err)
	}
	fmt.Println("done")
}

func openMigrations(dir string) (fs.FS, string, error) {
	if dir != "" {
		return os.DirFS(dir), dir, nil
	}
	fsys, err := migrations.FS()
	if err != nil {
		return nil, "", err
	}
	return fsys, "embed:migrations/meta", nil
}

func redactDSN(dsn string) string {
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
