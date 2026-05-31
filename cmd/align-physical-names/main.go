package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/solat/lowcode-database/internal/config"
	"github.com/solat/lowcode-database/internal/columntype"
	"github.com/solat/lowcode-database/internal/db"
	"github.com/solat/lowcode-database/internal/service"
	"github.com/solat/lowcode-database/internal/tenant"
)

// One-time utility: rename data DB columns from legacy pg_column (c_*) to logical name.
// For databases created before name=PG column unification; fresh installs use 000001_init.up.sql only.
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	meta, err := pgxpool.New(ctx, cfg.MetaDatabaseURL)
	if err != nil {
		log.Fatalf("meta pool: %v", err)
	}
	defer meta.Close()

	tm, err := db.NewTenantManager(ctx, cfg)
	if err != nil {
		log.Fatalf("tenant manager: %v", err)
	}
	defer tm.Close()

	rows, err := meta.Query(ctx, `
		SELECT c.tenant_id, t.schema_name, t.name AS table_name, c.name, c.pg_column, c.type_id
		FROM lc_columns c
		JOIN lc_tables t ON c.table_id = t.name AND c.tenant_id = t.tenant_id
		WHERE c.pg_column IS DISTINCT FROM c.name
		ORDER BY c.tenant_id, t.name, c.position`)
	if err != nil {
		log.Fatalf("query: %v", err)
	}
	defer rows.Close()

	type row struct {
		tenantID, schema, table, name, pgCol, typeID string
	}
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.tenantID, &r.schema, &r.table, &r.name, &r.pgCol, &r.typeID); err != nil {
			log.Fatalf("scan: %v", err)
		}
		if columntype.IsVirtual(r.typeID) {
			continue
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("rows: %v", err)
	}
	if len(pending) == 0 {
		log.Println("nothing to align")
		return
	}

	for _, r := range pending {
		if err := service.ValidateColumnName(r.name); err != nil {
			log.Fatalf("column %s.%s: %v", r.table, r.name, err)
		}
		tctx := tenant.WithTenantID(ctx, r.tenantID)
		data, err := tm.DataPool(tctx)
		if err != nil {
			log.Fatalf("tenant %s data pool: %v", r.tenantID, err)
		}
		sql := fmt.Sprintf(`ALTER TABLE %s.%s RENAME COLUMN %s TO %s`,
			pgx.Identifier{r.schema}.Sanitize(),
			pgx.Identifier{r.table}.Sanitize(),
			pgx.Identifier{r.pgCol}.Sanitize(),
			pgx.Identifier{r.name}.Sanitize(),
		)
		if _, err := data.Exec(ctx, sql); err != nil {
			log.Fatalf("rename %s.%s %s -> %s: %v", r.schema, r.table, r.pgCol, r.name, err)
		}
		if _, err := meta.Exec(ctx, `
			UPDATE lc_columns SET pg_column = name
			WHERE tenant_id = $1 AND table_id = $2 AND name = $3`,
			r.tenantID, r.table, r.name); err != nil {
			log.Fatalf("update meta %s.%s.%s: %v", r.tenantID, r.table, r.name, err)
		}
		log.Printf("aligned %s / %s.%s: %s -> %s", r.tenantID, r.schema, r.table, r.pgCol, r.name)
	}
	log.Printf("done (%d columns)", len(pending))
}
