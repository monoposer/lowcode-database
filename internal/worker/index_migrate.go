package worker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
)

// IndexMigrate reconciles lc_indexes migrate_status with shard DDL (CONCURRENTLY).
type IndexMigrate struct {
	Tenants  *postgres.TenantManager
	EventBus event.Bus
	Interval time.Duration
	Timeout  time.Duration
}

func (w *IndexMigrate) Run(ctx context.Context) {
	if w == nil || w.Tenants == nil {
		return
	}
	iv := w.Interval
	if iv <= 0 {
		iv = 10 * time.Second
	}
	t := time.NewTicker(iv)
	defer t.Stop()
	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *IndexMigrate) tick(ctx context.Context) {
	meta := w.Tenants.MetaPool()
	if meta == nil {
		return
	}
	// Also enqueue FTS indexes for columns with enable_fulltext.
	w.ensureFulltextIndexMeta(ctx)

	rows, err := meta.Query(ctx, `
		SELECT tenant_id, base_id, table_name, name, pg_index, vt_id::text, COALESCE(index_expr,''),
		       COALESCE(index_type,'btree'), COALESCE(is_unique,false), COALESCE(migrate_status,'pending')
		FROM lc_indexes
		WHERE migrate_status IN ('pending', 'drop_pending', 'error')
		ORDER BY updated_at ASC
		LIMIT 50`)
	if err != nil {
		return
	}
	defer rows.Close()

	type job struct {
		tenantID, baseID, tableName, name, pgIndex, vtID, expr, indexType, status string
		unique                                                                    bool
	}
	var jobs []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.tenantID, &j.baseID, &j.tableName, &j.name, &j.pgIndex, &j.vtID, &j.expr, &j.indexType, &j.unique, &j.status); err != nil {
			return
		}
		jobs = append(jobs, j)
	}
	for _, j := range jobs {
		w.apply(ctx, j.tenantID, j.baseID, j.tableName, j.name, j.pgIndex, j.vtID, j.expr, j.indexType, j.unique, j.status)
	}
}

func (w *IndexMigrate) ensureFulltextIndexMeta(ctx context.Context) {
	meta := w.Tenants.MetaPool()
	rows, err := meta.Query(ctx, `
		SELECT c.tenant_id, c.base_id, c.table_name, t.vt_id::text
		FROM lc_columns c
		JOIN lc_tables t ON t.tenant_id = c.tenant_id AND t.base_id = c.base_id AND t.name = c.table_name
		WHERE (c.config->>'enable_fulltext') IN ('true', '1')
		GROUP BY c.tenant_id, c.base_id, c.table_name, t.vt_id`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var tid, baseID, tableName, vtID string
		if err := rows.Scan(&tid, &baseID, &tableName, &vtID); err != nil {
			continue
		}
		name := "fts_" + tableName
		if len(name) > 40 {
			name = name[:40]
		}
		pgIndex := "idx_" + name
		expr := `to_tsvector('simple', COALESCE(data->>'_fulltext_text',''))`
		_, _ = meta.Exec(ctx, `
			INSERT INTO lc_indexes (
				tenant_id, base_id, table_name, name, pg_index, column_ids, is_unique,
				vt_id, index_expr, index_type, migrate_status
			) VALUES ($1, $2, $3, $4, $5, '[]'::jsonb, false, $6, $7, 'gin', 'pending')
			ON CONFLICT (tenant_id, base_id, table_name, name) DO NOTHING`,
			tid, baseID, tableName, name, pgIndex, vtID, expr)
	}
}

func (w *IndexMigrate) apply(ctx context.Context, tenantID, baseID, tableName, name, pgIndex, vtID, expr, indexType string, unique bool, status string) {
	timeout := w.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	meta := w.Tenants.MetaPool()
	_, _ = meta.Exec(ctx, `
		UPDATE lc_indexes SET migrate_status = 'building', updated_at = now()
		WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3 AND name = $4`, tenantID, baseID, tableName, name)

	pool, err := w.Tenants.PoolForTenant(ctx, tenantID)
	if err != nil {
		w.fail(ctx, tenantID, baseID, tableName, name, err)
		return
	}
	ctx, tables, err := w.Tenants.AttachDataTablesForTenant(ctx, tenantID)
	if err != nil {
		w.fail(ctx, tenantID, baseID, tableName, name, err)
		return
	}
	_ = postgres.EnsureDataTables(ctx, pool, tables)

	if status == "drop_pending" {
		sql := fmt.Sprintf(`DROP INDEX CONCURRENTLY IF EXISTS %s`, pgx.Identifier{pgIndex}.Sanitize())
		if _, err := pool.Exec(ctx, sql); err != nil {
			// Fallback without CONCURRENTLY
			sql = fmt.Sprintf(`DROP INDEX IF EXISTS %s`, pgx.Identifier{pgIndex}.Sanitize())
			if _, err2 := pool.Exec(ctx, sql); err2 != nil {
				w.fail(ctx, tenantID, baseID, tableName, name, err2)
				return
			}
		}
		_, _ = meta.Exec(ctx, `
			DELETE FROM lc_indexes WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3 AND name = $4`,
			tenantID, baseID, tableName, name)
		return
	}

	if expr == "" || vtID == "" {
		w.fail(ctx, tenantID, baseID, tableName, name, fmt.Errorf("missing index_expr or vt_id"))
		return
	}
	using := ""
	if strings.EqualFold(indexType, "gin") {
		using = "USING GIN "
	}
	uniqueSQL := ""
	if unique && using == "" {
		uniqueSQL = "UNIQUE "
	}
	sql := fmt.Sprintf(
		`CREATE %sINDEX CONCURRENTLY IF NOT EXISTS %s ON %s %s(%s) WHERE vt_id = %s`,
		uniqueSQL,
		pgx.Identifier{pgIndex}.Sanitize(),
		tables.QRecord(),
		using,
		expr,
		quoteLit(vtID),
	)
	if _, err := pool.Exec(ctx, sql); err != nil {
		sql2 := strings.Replace(sql, " CONCURRENTLY", "", 1)
		if _, err2 := pool.Exec(ctx, sql2); err2 != nil {
			w.fail(ctx, tenantID, baseID, tableName, name, err2)
			return
		}
		sql = sql2
	}
	if w.EventBus != nil {
		_ = w.EventBus.Publish(ctx, event.Envelope{
			ID:        uuid.NewString(),
			Type:      event.BusType(event.MetadataIndexCreated),
			TenantID:  tenantID,
			TableName: tableName,
			Data:      map[string]any{"index": map[string]any{"name": name}, "ddl": sql},
			Time:      time.Now().UTC(),
		})
	}
	_, _ = meta.Exec(ctx, `
		UPDATE lc_indexes SET migrate_status = 'ready', migrate_error = '', updated_at = now()
		WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3 AND name = $4`, tenantID, baseID, tableName, name)
}

func (w *IndexMigrate) fail(ctx context.Context, tenantID, baseID, tableName, name string, err error) {
	log.Printf("index migrate %s.%s: %v", tableName, name, err)
	_, _ = w.Tenants.MetaPool().Exec(ctx, `
		UPDATE lc_indexes SET migrate_status = 'error', migrate_error = $5, updated_at = now()
		WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3 AND name = $4`,
		tenantID, baseID, tableName, name, err.Error())
}

func quoteLit(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
