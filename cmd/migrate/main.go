package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/solat/lowcode-database/internal/config"
	"github.com/solat/lowcode-database/internal/migrator"
)

func main() {
	var (
		target = flag.String("target", "meta", "migration target: meta | data")
		dir    = flag.String("dir", "", "migrations directory (default: docker/postgres/migrations/{target})")
		dsn    = flag.String("database-url", "", "postgres URL (overrides env)")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	migDir := *dir
	if migDir == "" {
		var err error
		migDir, err = migrator.DefaultDir(*target)
		if err != nil {
			log.Fatalf("migrations dir: %v", err)
		}
	}

	dbURL := *dsn
	if dbURL == "" {
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("config: %v", err)
		}
		switch *target {
		case "meta":
			dbURL = cfg.MetaDatabaseURL
		case "data":
			dbURL = firstNonEmpty(os.Getenv("DATA_DATABASE_URL"), cfg.DefaultTenantDataDSN)
		default:
			log.Fatalf("unknown target %q (use meta or data)", *target)
		}
	}
	if dbURL == "" {
		log.Fatalf("database URL required: pass -database-url or set META_DATABASE_URL / DEFAULT_TENANT_DATA_DSN")
	}

	fmt.Printf("migrating %s database: %s\n", *target, redactDSN(dbURL))
	fmt.Printf("migrations: %s\n", migDir)

	if err := migrator.Apply(ctx, dbURL, migDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	fmt.Println("done")
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
