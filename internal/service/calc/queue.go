package calc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
)

func qTbl(ctx context.Context) string   { return postgres.TablesFromContext(ctx).QCalcQueue() }
func dlqTbl(ctx context.Context) string { return postgres.TablesFromContext(ctx).QCalcDLQ() }

// Enqueue inserts a pending calc_queue row unless one already exists for (tenant, record) with status=pending.
// Does not modify record / version.
func Enqueue(ctx context.Context, pool *pgxpool.Pool, tenantID, tableName, recordID string, targetFieldIDs []string) error {
	if pool == nil || tenantID == "" || recordID == "" {
		return nil
	}
	var ids any
	if len(targetFieldIDs) > 0 {
		ids = targetFieldIDs
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (tenant_id, table_name, record_id, target_field_ids, status, next_run_at)
		SELECT $1, $2, $3, $4, 0, now()
		WHERE NOT EXISTS (
			SELECT 1 FROM %s
			WHERE tenant_id = $1 AND record_id = $3 AND status = 0
		)`, qTbl(ctx), qTbl(ctx)), tenantID, tableName, recordID, ids)
	return err
}

// EnqueueTable queues calc for every row of a logical table (virtual column add/update backfill).
// Existing pending jobs on that table are widened to a full-field recalc so the new column is included.
func EnqueueTable(ctx context.Context, pool *pgxpool.Pool, tenantID, tableName, vtID string, fieldIDs []string) error {
	if pool == nil || tenantID == "" || tableName == "" || vtID == "" {
		return nil
	}
	q := qTbl(ctx)
	rec := postgres.TablesFromContext(ctx).QRecord()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s SET target_field_ids = NULL
		WHERE tenant_id = $1 AND table_name = $2 AND status = 0`, q), tenantID, tableName); err != nil {
		return err
	}
	var ids any
	if len(fieldIDs) > 0 {
		ids = fieldIDs
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (tenant_id, table_name, record_id, target_field_ids, status, next_run_at)
		SELECT $1, $2, r.record_id, $3, 0, now()
		FROM %s r
		WHERE r.tenant_id = $1 AND r.vt_id = $4::uuid
		  AND NOT EXISTS (
			SELECT 1 FROM %s q
			WHERE q.tenant_id = r.tenant_id AND q.record_id = r.record_id AND q.status = 0
		  )`, q, rec, q), tenantID, tableName, ids, vtID)
	return err
}

// EnqueueMany fans out with per-record dedup.
func EnqueueMany(ctx context.Context, pool *pgxpool.Pool, jobs []Task) error {
	for _, j := range jobs {
		if err := Enqueue(ctx, pool, j.TenantID, j.TableName, j.RecordID, j.TargetFieldIDs); err != nil {
			return err
		}
	}
	return nil
}

// Claim locks a batch of due pending tasks (FOR UPDATE SKIP LOCKED) and marks them processing.
func Claim(ctx context.Context, pool *pgxpool.Pool, batch int) ([]Task, error) {
	return ClaimFair(ctx, pool, batch, 0)
}

// ClaimFair is Claim with a per-tenant cap so one tenant cannot fill the worker.
func ClaimFair(ctx context.Context, pool *pgxpool.Pool, batch, perTenant int) ([]Task, error) {
	if batch <= 0 {
		batch = 16
	}
	if perTenant <= 0 {
		perTenant = batch
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		WITH due AS (
			SELECT id, tenant_id, table_name, record_id, target_field_ids, status, retry_count, max_retry
			FROM %s
			WHERE status = 0 AND next_run_at <= now()
			ORDER BY id ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		),
		ranked AS (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY tenant_id ORDER BY id) AS rn
			FROM due
		)
		SELECT id, tenant_id, table_name, record_id, target_field_ids, status, retry_count, max_retry
		FROM ranked
		WHERE rn <= $2
		ORDER BY id ASC
		LIMIT $3`, qTbl(ctx)), batch*4, perTenant, batch)
	if err != nil {
		return nil, err
	}
	var out []Task
	for rows.Next() {
		var t Task
		var fields []string
		if err := rows.Scan(&t.ID, &t.TenantID, &t.TableName, &t.RecordID, &fields, &t.Status, &t.RetryCount, &t.MaxRetry); err != nil {
			rows.Close()
			return nil, err
		}
		t.TargetFieldIDs = fields
		out = append(out, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, tx.Commit(ctx)
	}
	ids := make([]int64, len(out))
	for i, t := range out {
		ids[i] = t.ID
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET status = 1 WHERE id = ANY($1)`, qTbl(ctx)), ids); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func MarkCompleted(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, qTbl(ctx)), id)
	return err
}

func MarkRetry(ctx context.Context, pool *pgxpool.Pool, t Task, cause error) error {
	next := t.RetryCount + 1
	msg := ""
	if cause != nil {
		msg = cause.Error()
		if len(msg) > 2000 {
			msg = msg[:2000]
		}
	}
	if next >= t.MaxRetry && t.MaxRetry > 0 {
		_, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s (queue_id, tenant_id, table_name, record_id, target_field_ids, retry_count, last_error)
			SELECT id, tenant_id, table_name, record_id, target_field_ids, $2, $3
			FROM %s WHERE id = $1`, dlqTbl(ctx), qTbl(ctx)), t.ID, next, msg)
		if err != nil {
			return err
		}
		_, err = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, qTbl(ctx)), t.ID)
		return err
	}
	backoff := time.Duration(1<<min(next, 8)) * time.Second
	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET status = 0, retry_count = $2, next_run_at = now() + $3::interval, last_error = $4
		WHERE id = $1`, qTbl(ctx)), t.ID, next, fmt.Sprintf("%d seconds", int(backoff.Seconds())), msg)
	return err
}

// PendingRecordIDs returns record ids in the given set that have a pending (status=0) task.
func PendingRecordIDs(ctx context.Context, pool *pgxpool.Pool, recordIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(recordIDs) == 0 {
		return out, nil
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT record_id FROM %s
		WHERE record_id = ANY($1) AND status = 0`, qTbl(ctx)), recordIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func HasPending(ctx context.Context, pool *pgxpool.Pool, recordID string) (bool, error) {
	var n int
	err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::int FROM %s WHERE record_id = $1 AND status = 0`, qTbl(ctx)), recordID).Scan(&n)
	return n > 0, err
}

// PurgeFinished deletes completed and exhausted queue rows left by older workers.
func PurgeFinished(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return nil
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE status IN (2, 3)`, qTbl(ctx)))
	return err
}

// ReplayDeadLetters re-enqueues failed tasks from calc_dead_letter (status pending).
func ReplayDeadLetters(ctx context.Context, pool *pgxpool.Pool, tenantID string, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, table_name, record_id, target_field_ids
		FROM %s
		WHERE ($1 = '' OR tenant_id = $1)
		ORDER BY failed_at ASC
		LIMIT $2`, dlqTbl(ctx)), tenantID, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type row struct {
		id        int64
		tenantID  string
		tableName string
		recordID  string
		fields    []string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.tenantID, &r.tableName, &r.recordID, &r.fields); err != nil {
			return 0, err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	n := 0
	for _, r := range list {
		if err := Enqueue(ctx, pool, r.tenantID, r.tableName, r.recordID, r.fields); err != nil {
			return n, err
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, dlqTbl(ctx)), r.id); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// QueueSnapshot is queue length / failure / per-tenant pending counts.
type QueueSnapshot struct {
	Pending     int            `json:"pending"`
	Processing  int            `json:"processing"`
	Completed   int            `json:"completed"`
	Failed      int            `json:"failed"`
	DeadLetters int            `json:"deadLetters"`
	ByTenant    map[string]int `json:"pendingByTenant"`
	Alert       bool           `json:"alert"`
}

func Snapshot(ctx context.Context, pool *pgxpool.Pool, alertLen int) (QueueSnapshot, error) {
	var s QueueSnapshot
	s.ByTenant = map[string]int{}
	if pool == nil {
		return s, fmt.Errorf("pool required")
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT status, COUNT(*)::int FROM %s GROUP BY status`, qTbl(ctx)))
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var st, n int
		if err := rows.Scan(&st, &n); err != nil {
			rows.Close()
			return s, err
		}
		switch int16(st) {
		case StatusPending:
			s.Pending = n
		case StatusProcessing:
			s.Processing = n
		case StatusCompleted:
			s.Completed = n
		case StatusFailed:
			s.Failed = n
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return s, err
	}
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s`, dlqTbl(ctx))).Scan(&s.DeadLetters)
	trows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT tenant_id, COUNT(*)::int FROM %s WHERE status = 0 GROUP BY tenant_id`, qTbl(ctx)))
	if err != nil {
		return s, err
	}
	defer trows.Close()
	for trows.Next() {
		var tid string
		var n int
		if err := trows.Scan(&tid, &n); err != nil {
			return s, err
		}
		s.ByTenant[tid] = n
	}
	if alertLen > 0 && s.Pending >= alertLen {
		s.Alert = true
	}
	return s, trows.Err()
}
