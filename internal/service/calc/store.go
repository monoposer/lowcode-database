package calc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/internal/infra/postgres"
)

func recordTbl(ctx context.Context) string {
	return postgres.TablesFromContext(ctx).QRecord()
}

func ReadRecord(ctx context.Context, pool *pgxpool.Pool, tenantID, vtID, recordID string) (*Rec, error) {
	var data []byte
	rec := &Rec{ID: recordID, VTID: vtID, WSID: tenantID, Data: map[string]any{}}
	err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT data, version FROM %s WHERE vt_id = $1 AND tenant_id = $2 AND record_id = $3`, recordTbl(ctx)),
		vtID, tenantID, recordID).Scan(&data, &rec.Version)
	if err != nil {
		return nil, err
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &rec.Data)
	}
	if rec.Data == nil {
		rec.Data = map[string]any{}
	}
	return rec, nil
}

func ReadRecords(ctx context.Context, pool *pgxpool.Pool, tenantID, vtID string, ids []string) (map[string]*Rec, error) {
	out := map[string]*Rec{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT record_id, data, version FROM %s
		WHERE vt_id = $1 AND tenant_id = $2 AND record_id = ANY($3)`, recordTbl(ctx)),
		vtID, tenantID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var data []byte
		var ver int64
		if err := rows.Scan(&id, &data, &ver); err != nil {
			return nil, err
		}
		m := map[string]any{}
		_ = json.Unmarshal(data, &m)
		out[id] = &Rec{ID: id, VTID: vtID, WSID: tenantID, Data: m, Version: ver}
	}
	return out, rows.Err()
}

// PatchCalcFields writes only calc field keys with optimistic locking. Returns false on version conflict.
func PatchCalcFields(ctx context.Context, pool *pgxpool.Pool, rec *Rec, patched map[string]any) (bool, error) {
	merged := map[string]any{}
	for k, v := range rec.Data {
		merged[k] = v
	}
	for k, v := range patched {
		merged[k] = v
	}
	payload, err := json.Marshal(merged)
	if err != nil {
		return false, err
	}
	tag, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s SET data = $1::jsonb, version = version + 1, updated_at = now()
		WHERE vt_id = $2 AND tenant_id = $3 AND record_id = $4 AND version = $5`, recordTbl(ctx)),
		payload, rec.VTID, rec.WSID, rec.ID, rec.Version)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ErrVersionConflict is returned when optimistic lock fails.
var ErrVersionConflict = fmt.Errorf("record version conflict")
