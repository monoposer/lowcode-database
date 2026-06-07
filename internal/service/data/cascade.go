package data

import (
	"context"
	"encoding/json"
	"fmt"
)

// cascadeFulltextDependents refreshes _fulltext_text on rows that lookup this source record.
func (s *Data) cascadeFulltextDependents(ctx context.Context, sourceTable, sourceRecordID string) {
	if !s.B.IsVirtualRecordsMode() || sourceRecordID == "" {
		return
	}
	ctx, tables, err := s.B.Tenants.AttachDataTables(ctx)
	if err != nil {
		return
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return
	}
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return
	}
	meta := s.B.Tenants.MetaPool()
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return
	}
	rows, err := meta.Query(ctx, `
		SELECT c.table_id, c.config, COALESCE(t.vt_id,'')
		FROM lc_columns c
		JOIN lc_tables t ON t.tenant_id = c.tenant_id AND t.base_id = c.base_id AND t.name = c.table_id
		WHERE c.tenant_id = $1 AND c.base_id = $2 AND c.type_id = 'lookup'
		  AND (c.config->>'enable_fulltext') IN ('true','1')
		  AND COALESCE(t.vt_id,'') <> ''`, tid, baseID)
	if err != nil {
		return
	}
	defer rows.Close()
	pool, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return
	}
	tbl := tables.QRecord()
	for rows.Next() {
		var depTable, depVT string
		var cfgRaw []byte
		if err := rows.Scan(&depTable, &cfgRaw, &depVT); err != nil {
			continue
		}
		cfg := map[string]any{}
		_ = json.Unmarshal(cfgRaw, &cfg)
		relCol := configString(cfg, "relation_column_id")
		var relCfgRaw []byte
		if err := meta.QueryRow(ctx, `
			SELECT config FROM lc_columns WHERE tenant_id = $1 AND base_id = $2 AND table_id = $3
			  AND (name = $4 OR id::text = $4) AND type_id IN ('link','relationship','relation_fk')`,
			tid, baseID, depTable, relCol).Scan(&relCfgRaw); err != nil {
			continue
		}
		relCfg := map[string]any{}
		_ = json.Unmarshal(relCfgRaw, &relCfg)
		if configString(relCfg, "target_table_id") != sourceTable {
			continue
		}
		fk := configString(relCfg, "target_column_id")
		if fk == "" {
			continue
		}
		q, err := pool.Query(ctx, fmt.Sprintf(`
			SELECT record_id, data FROM %s WHERE vt_id = $1 AND tenant_id = $2 AND data->>$3 = $4`, tbl),
			depVT, tenantID, fk, sourceRecordID)
		if err != nil {
			continue
		}
		for q.Next() {
			var rid string
			var dataJSON []byte
			if err := q.Scan(&rid, &dataJSON); err != nil {
				continue
			}
			native := map[string]any{}
			_ = json.Unmarshal(dataJSON, &native)
			native = s.applyFulltextOnWrite(ctx, depTable, nil, native)
			payload, _ := json.Marshal(native)
			_, _ = pool.Exec(ctx, fmt.Sprintf(`
				UPDATE %s SET data = $1::jsonb, updated_at = now()
				WHERE vt_id = $2 AND tenant_id = $3 AND record_id = $4`, tbl),
				payload, depVT, tenantID, rid)
		}
		q.Close()
	}
}
