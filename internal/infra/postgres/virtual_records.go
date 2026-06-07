package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Record store (NocoBase-style unified JSONB rows).
// Physical columns keep record_id / tenant_id / vt_id for LIST partition + shard routing.
// Spec mapping: id=record_id, tenant_id=tenant_id, table_id=vt_id.
const (
	RecordTable = "record"
	// VirtualRecordsTable is the historical name; same relation as RecordTable.
	VirtualRecordsTable = RecordTable
	LinkRefTable        = "link_ref"
	CalcQueueTable      = "calc_queue"
)

var nonIdent = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// EnsureVirtualRecordsParent creates the shared LIST-partitioned record parent (idempotent).
func EnsureVirtualRecordsParent(ctx context.Context, pool *pgxpool.Pool) error {
	return EnsureDataTables(ctx, pool, SharedDataTables())
}

// EnsureDataTables creates the LIST-partitioned record parent, link_ref, and calc_queue for a store.
func EnsureDataTables(ctx context.Context, pool *pgxpool.Pool, tables DataTables) error {
	if pool == nil {
		return fmt.Errorf("pool is required")
	}
	if tables.Record == "" {
		tables = SharedDataTables()
	}
	if tables.Shared() {
		if err := migrateVirtualRecordsRename(ctx, pool); err != nil {
			return err
		}
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
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				record_id       TEXT NOT NULL,
				tenant_id           TEXT NOT NULL,
				vt_id           TEXT NOT NULL,
				data            JSONB NOT NULL DEFAULT '{}'::jsonb,
				version         BIGINT NOT NULL DEFAULT 1,
				created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				created_by      TEXT,
				updated_by      TEXT,
				PRIMARY KEY (vt_id, record_id)
			) PARTITION BY LIST (vt_id)`, tbl),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1`, tbl),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (tenant_id)`, idxRecordTenant, tbl),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (tenant_id, vt_id)`, idxRecordTable, tbl),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (created_at)`, idxRecordCtime, tbl),
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id              BIGSERIAL PRIMARY KEY,
				tenant_id       TEXT NOT NULL,
				from_table_id   TEXT NOT NULL,
				from_record_id  TEXT NOT NULL,
				from_field_id   TEXT NOT NULL,
				to_table_id     TEXT NOT NULL,
				to_record_id    TEXT NOT NULL,
				created_at      TIMESTAMPTZ DEFAULT NOW(),
				UNIQUE (from_record_id, from_field_id, to_record_id)
			)`, link),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (from_table_id, from_record_id, from_field_id)`, idxLinkFrom, link),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (to_table_id, to_record_id)`, idxLinkTo, link),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (tenant_id)`, idxLinkTenant, link),
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id               BIGSERIAL PRIMARY KEY,
				tenant_id        TEXT NOT NULL,
				table_id         TEXT NOT NULL,
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
				table_id         TEXT NOT NULL,
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

func migrateVirtualRecordsRename(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM pg_class c
				JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE n.nspname = 'public' AND c.relname = 'virtual_records' AND c.relkind IN ('p','r')
			) AND NOT EXISTS (
				SELECT 1 FROM pg_class c
				JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE n.nspname = 'public' AND c.relname = 'record' AND c.relkind IN ('p','r')
			) THEN
				ALTER TABLE virtual_records RENAME TO record;
			END IF;
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = 'record' AND column_name = 'ws_id'
			) THEN
				ALTER TABLE record RENAME COLUMN ws_id TO tenant_id;
			END IF;
		END $$`)
	return err
}

// PartitionTableName returns a safe partition relation name for vt_id.
func PartitionTableName(vtID string) string {
	s := nonIdent.ReplaceAllString(strings.ToLower(vtID), "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "x"
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return "prt_" + s
}

// EnsureVirtualRecordsPartition creates a LIST partition on the ctx store (or shared).
func EnsureVirtualRecordsPartition(ctx context.Context, pool *pgxpool.Pool, vtID string) error {
	return EnsureVirtualRecordsPartitionOn(ctx, pool, TablesFromContext(ctx), vtID)
}

// EnsureVirtualRecordsPartitionOn creates a LIST partition for vt_id on the given parent (idempotent).
func EnsureVirtualRecordsPartitionOn(ctx context.Context, pool *pgxpool.Pool, tables DataTables, vtID string) error {
	if pool == nil {
		return fmt.Errorf("pool is required")
	}
	vtID = strings.TrimSpace(vtID)
	if vtID == "" {
		return fmt.Errorf("vt_id is required")
	}
	if tables.Record == "" {
		tables = SharedDataTables()
	}
	if err := EnsureDataTables(ctx, pool, tables); err != nil {
		return err
	}
	part := PartitionTableName(vtID)
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_inherits i
			JOIN pg_class c ON c.oid = i.inhrelid
			JOIN pg_class p ON p.oid = i.inhparent
			WHERE p.relname = $1 AND c.relname = $2
		)`, tables.Record, part).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	sql := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES IN (%s)`,
		pgx.Identifier{part}.Sanitize(),
		tables.QRecord(),
		quoteLiteral(vtID),
	)
	if _, err := pool.Exec(ctx, sql); err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "overlaps") {
			return nil
		}
		return fmt.Errorf("create partition %s: %w", part, err)
	}
	return nil
}

// DropVirtualRecordsPartition drops the LIST partition for vt_id if present.
func DropVirtualRecordsPartition(ctx context.Context, pool *pgxpool.Pool, vtID string) error {
	_, err := DropVirtualRecordsPartitionSQL(ctx, pool, vtID)
	return err
}

// DropVirtualRecordsPartitionSQL drops the LIST partition and returns the executed DDL.
func DropVirtualRecordsPartitionSQL(ctx context.Context, pool *pgxpool.Pool, vtID string) (string, error) {
	if pool == nil || strings.TrimSpace(vtID) == "" {
		return "", nil
	}
	part := PartitionTableName(vtID)
	ddl := fmt.Sprintf(`DROP TABLE IF EXISTS %s`, pgx.Identifier{part}.Sanitize())
	_, err := pool.Exec(ctx, ddl)
	return ddl, err
}

func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// EnsureRLSSharedInfrastructure is the VR-mode entry used by existing call sites.
func EnsureRLSSharedInfrastructure(ctx context.Context, pool *pgxpool.Pool) error {
	return EnsureVirtualRecordsParent(ctx, pool)
}

// WithRLSTenantSession runs fn in a transaction (legacy name). VR mode does not set RLS GUC;
// callers must filter by tenant_id in SQL.
func WithRLSTenantSession(ctx context.Context, pool *pgxpool.Pool, tenantID string, fn func(pgx.Tx) error) error {
	if tenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CountVirtualRecordPartitions returns the number of LIST partitions of record.
func CountVirtualRecordPartitions(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM pg_inherits i
		JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname = $1`, TablesFromContext(ctx).Record).Scan(&n)
	return n, err
}
