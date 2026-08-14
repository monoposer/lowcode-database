package calc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/internal/service/shared"
)

type tableMeta struct {
	Name   string
	VTID   string
	Fields []Field
}

func loadTableMeta(ctx context.Context, meta *pgxpool.Pool, tenantID, tableRef string) (*tableMeta, error) {
	var name, vtID, baseID string
	err := meta.QueryRow(ctx, `
		SELECT name, vt_id::text, base_id FROM lc_tables
		WHERE tenant_id = $1 AND (name = $2 OR vt_id::text = $2)
		ORDER BY created_at ASC LIMIT 1`, tenantID, tableRef).Scan(&name, &vtID, &baseID)
	if err != nil {
		return nil, fmt.Errorf("load table %q: %w", tableRef, err)
	}
	rows, err := meta.Query(ctx, `
		SELECT id::text, name, type_id, config FROM lc_columns
		WHERE tenant_id = $1 AND base_id = $2 AND table_name = $3
		ORDER BY position, name`, tenantID, baseID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tm := &tableMeta{Name: name, VTID: vtID}
	for rows.Next() {
		var f Field
		var cfg []byte
		if err := rows.Scan(&f.ID, &f.Name, &f.TypeID, &cfg); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(cfg, &f.Config)
		if f.Config == nil {
			f.Config = map[string]any{}
		}
		tm.Fields = append(tm.Fields, f)
	}
	return tm, rows.Err()
}

func loadVTMap(ctx context.Context, meta *pgxpool.Pool, tenantID string) (map[string]string, error) {
	rows, err := meta.Query(ctx, `
		SELECT name, vt_id::text FROM lc_tables WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, vt string
		if err := rows.Scan(&name, &vt); err != nil {
			return nil, err
		}
		out[name] = vt
	}
	return out, rows.Err()
}

func lookupTargetTable(f Field, linkFields []Field) string {
	if t := firstCfg(f.Config, "to_table_name", "target_table_name"); t != "" {
		return t
	}
	linkName := firstCfg(f.Config, "link_field_id", "relation_column_id")
	for _, lf := range linkFields {
		if lf.Name == linkName || lf.ID == linkName {
			return firstCfg(lf.Config, "to_table_name", "target_table_name")
		}
	}
	return ""
}

func enrichLookupRollupTargets(fields []Field) []Field {
	var links []Field
	for _, f := range fields {
		if IsLinkType(f.TypeID) {
			links = append(links, f)
		}
	}
	out := make([]Field, len(fields))
	copy(out, fields)
	for i, f := range out {
		if f.TypeID != "lookup" && f.TypeID != "rollup" {
			continue
		}
		if firstCfg(f.Config, "to_table_name", "target_table_name") == "" {
			if t := lookupTargetTable(f, links); t != "" {
				if f.Config == nil {
					f.Config = map[string]any{}
				}
				f.Config["to_table_name"] = t
				out[i] = f
			}
		}
	}
	return out
}

func fieldDeps(f Field) []string {
	raw, ok := f.Config["deps"]
	if ok {
		switch t := raw.(type) {
		case []any:
			var out []string
			for _, x := range t {
				out = append(out, fmt.Sprint(x))
			}
			if len(out) > 0 {
				return out
			}
		case []string:
			if len(t) > 0 {
				return t
			}
		}
	}
	switch f.TypeID {
	case "formula":
		return nil // filled from expression at enqueue time
	case "lookup", "rollup":
		return []string{
			firstCfg(f.Config, "link_field_id", "relation_column_id"),
			firstCfg(f.Config, "target_field_id", "target_column_id"),
		}
	}
	return nil
}

var _ = shared.CfgString
