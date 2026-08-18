package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Record store (NocoBase-style unified JSONB rows).
// Physical columns keep record_id / tenant_id / vt_id for shard routing and per-table filters.
// Spec mapping: id=record_id, tenant_id=tenant_id, table_name=vt_id.
const (
	RecordTable    = "record"
	LinkRefTable   = "link_ref"
	CalcQueueTable = "calc_queue"
)

// EnsureVirtualRecordsParent creates the shared record table (idempotent).
func EnsureVirtualRecordsParent(ctx context.Context, pool *pgxpool.Pool) error {
	return EnsureDataTables(ctx, pool, SharedDataTables())
}

// EnsureDataTables creates record, link_ref, and calc_queue for a store.
func EnsureDataTables(ctx context.Context, pool *pgxpool.Pool, tables DataTables) error {
	if pool == nil {
		return fmt.Errorf("pool is required")
	}
	if tables.Record == "" {
		tables = SharedDataTables()
	}
	tbl := tables.QRecord()
	link := tables.QLinkRef()
	queue := tables.QCalcQueue()
	dlq := tables.QCalcDLQ()
	idxRecordTenant := tables.indexIdent(indexRelName(tables.Record, "tenant"))
	idxRecordTable := tables.indexIdent(indexRelName(tables.Record, "table"))
	idxRecordCtime := tables.indexIdent(indexRelName(tables.Record, "ctime"))
	idxLinkFrom := tables.indexIdent(indexRelName(tables.LinkRef, "from"))
	idxLinkTo := tables.indexIdent(indexRelName(tables.LinkRef, "to"))
	idxLinkTenant := tables.indexIdent(indexRelName(tables.LinkRef, "tenant"))
	idxCalcNext := tables.indexIdent(indexRelName(tables.CalcQueue, "nextrun"))
	idxCalcRec := tables.indexIdent(indexRelName(tables.CalcQueue, "record"))
	idxCalcPend := tables.indexIdent(indexRelName(tables.CalcQueue, "pending"))
	idxDLQ := tables.indexIdent(indexRelName(tables.CalcDLQ, "tenant"))
	if tables.Shared() {
		idxRecordTenant = pgx.Identifier{"idx_record_tenant"}.Sanitize()
		idxRecordTable = pgx.Identifier{"idx_record_table"}.Sanitize()
		idxRecordCtime = pgx.Identifier{"idx_record_ctime"}.Sanitize()
		idxLinkFrom = pgx.Identifier{"idx_link_ref_from"}.Sanitize()
		idxLinkTo = pgx.Identifier{"idx_link_ref_to"}.Sanitize()
		idxLinkTenant = pgx.Identifier{"idx_link_ref_tenant"}.Sanitize()
		idxCalcNext = pgx.Identifier{"idx_calc_status_nextrun"}.Sanitize()
		idxCalcRec = pgx.Identifier{"idx_calc_record"}.Sanitize()
		idxCalcPend = pgx.Identifier{"idx_calc_tenant_pending"}.Sanitize()
		idxDLQ = pgx.Identifier{"idx_calc_dlq_tenant"}.Sanitize()
	}
	fnName := "fn_clean_link_ref_on_record_delete"
	trgName := "trg_record_del_link_ref"
	if !tables.Shared() {
		fnName = "fn_clean_link_ref_" + dedicatedPrefix(tables.TenantID)
		if len(fnName) > 63 {
			fnName = fnName[:63]
		}
		trgName = "trg_" + dedicatedPrefix(tables.TenantID) + "_del_link_ref"
		if len(trgName) > 63 {
			trgName = trgName[:63]
		}
	}
	fnIdent := pgx.Identifier{fnName}.Sanitize()
	trgIdent := pgx.Identifier{trgName}.Sanitize()
	_, _ = pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pg_stat_statements`)
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				record_id       TEXT NOT NULL,
				tenant_id           TEXT NOT NULL,
				vt_id           UUID NOT NULL,
				data            JSONB NOT NULL DEFAULT '{}'::jsonb,
				version         BIGINT NOT NULL DEFAULT 1,
				created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				created_by      TEXT,
				updated_by      TEXT,
				PRIMARY KEY (vt_id, record_id)
			)`, tbl),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1`, tbl),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (tenant_id)`, idxRecordTenant, tbl),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (tenant_id, vt_id)`, idxRecordTable, tbl),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (created_at)`, idxRecordCtime, tbl),
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id              BIGSERIAL PRIMARY KEY,
				tenant_id       TEXT NOT NULL,
				from_table_name   TEXT NOT NULL,
				from_record_id  TEXT NOT NULL,
				from_field_id   TEXT NOT NULL,
				to_table_name     TEXT NOT NULL,
				to_record_id    TEXT NOT NULL,
				created_at      TIMESTAMPTZ DEFAULT NOW(),
				UNIQUE (from_record_id, from_field_id, to_record_id)
			)`, link),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (from_table_name, from_record_id, from_field_id)`, idxLinkFrom, link),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (to_table_name, to_record_id)`, idxLinkTo, link),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (tenant_id)`, idxLinkTenant, link),
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id               BIGSERIAL PRIMARY KEY,
				tenant_id        TEXT NOT NULL,
				table_name         TEXT NOT NULL,
				record_id        TEXT NOT NULL,
				target_field_ids TEXT[],
				status           SMALLINT NOT NULL DEFAULT 0,
				retry_count      INT NOT NULL DEFAULT 0,
				max_retry        INT NOT NULL DEFAULT 8,
				next_run_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				created_at       TIMESTAMPTZ DEFAULT NOW(),
				completed_at     TIMESTAMPTZ,
				last_error       TEXT
			)`, queue),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (status, next_run_at)`, idxCalcNext, queue),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (record_id, status)`, idxCalcRec, queue),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (tenant_id, record_id) WHERE status = 0`, idxCalcPend, queue),
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id               BIGSERIAL PRIMARY KEY,
				queue_id         BIGINT,
				tenant_id        TEXT NOT NULL,
				table_name         TEXT NOT NULL,
				record_id        TEXT NOT NULL,
				target_field_ids TEXT[],
				retry_count      INT NOT NULL DEFAULT 0,
				last_error       TEXT,
				failed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`, dlq),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (tenant_id, failed_at DESC)`, idxDLQ, dlq),
		fmt.Sprintf(`
			CREATE OR REPLACE FUNCTION %s() RETURNS TRIGGER AS $$
			BEGIN
				DELETE FROM %s WHERE from_record_id = OLD.record_id OR to_record_id = OLD.record_id;
				RETURN OLD;
			END;
			$$ LANGUAGE plpgsql`, fnIdent, link),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("record ddl: %w", err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		DROP TRIGGER IF EXISTS %s ON %s;
		CREATE TRIGGER %s
		AFTER DELETE ON %s
		FOR EACH ROW EXECUTE FUNCTION %s()`, trgIdent, tbl, trgIdent, tbl, fnIdent)); err != nil {
		return fmt.Errorf("record delete trigger: %w", err)
	}
	return nil
}

// DeleteRecordsByVTID deletes rows for a logical table and returns the executed SQL.
func DeleteRecordsByVTID(ctx context.Context, pool *pgxpool.Pool, tenantID, vtID string) (string, error) {
	if pool == nil || strings.TrimSpace(vtID) == "" {
		return "", nil
	}
	tables := TablesFromContext(ctx)
	if tables.Record == "" {
		tables = SharedDataTables()
	}
	q := tables.QRecord()
	if strings.TrimSpace(tenantID) != "" {
		sql := fmt.Sprintf(`DELETE FROM %s WHERE tenant_id = $1 AND vt_id = $2`, q)
		_, err := pool.Exec(ctx, sql, tenantID, vtID)
		return sql, err
	}
	sql := fmt.Sprintf(`DELETE FROM %s WHERE vt_id = $1`, q)
	_, err := pool.Exec(ctx, sql, vtID)
	return sql, err
}
