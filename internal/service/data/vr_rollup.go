package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/internal/infra/postgres"
)

// refreshRollupsAfterChildWrite updates parent rows' data._rollup_* when a child row changes.
func (s *Data) refreshRollupsAfterChildWrite(ctx context.Context, childTableID, _ string, childData map[string]any) error {
	if !s.B.IsVirtualRecordsMode() {
		return nil
	}
	ctx, tables, err := s.B.Tenants.AttachDataTables(ctx)
	if err != nil {
		return err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return err
	}
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return err
	}
	meta := s.B.Tenants.MetaPool()
	pool, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return err
	}
	childVT, err := s.B.Tenants.TableVTID(ctx, tenantID, baseID, childTableID)
	if err != nil {
		return err
	}

	rows, err := meta.Query(ctx, `
		SELECT c.table_id, c.name, c.config, COALESCE(t.vt_id, '')
		FROM lc_columns c
		JOIN lc_tables t ON t.tenant_id = c.tenant_id AND t.base_id = c.base_id AND t.name = c.table_id
		WHERE c.tenant_id = $1 AND c.base_id = $2 AND c.type_id = 'rollup'
	`, tid, baseID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var parentTable, colName, parentVT string
		var cfgRaw []byte
		if err := rows.Scan(&parentTable, &colName, &cfgRaw, &parentVT); err != nil {
			return err
		}
		if parentVT == "" {
			continue
		}
		cfg := map[string]any{}
		_ = json.Unmarshal(cfgRaw, &cfg)
		relCol := configString(cfg, "relation_column_id")
		if relCol == "" {
			continue
		}
		var relCfgRaw []byte
		err := meta.QueryRow(ctx, `
			SELECT config FROM lc_columns
			WHERE tenant_id = $1 AND base_id = $2 AND table_id = $3 AND (name = $4 OR id::text = $4) AND type_id IN ('link','relationship','relation_fk')
		`, tid, baseID, parentTable, relCol).Scan(&relCfgRaw)
		if err != nil {
			continue
		}
		relCfg := map[string]any{}
		_ = json.Unmarshal(relCfgRaw, &relCfg)
		target := configString(relCfg, "target_table_id")
		if target != childTableID {
			continue
		}
		childLink := configString(relCfg, "link_column_id")
		agg := strings.ToLower(configString(cfg, "aggregate"))
		if agg == "" {
			agg = "count"
		}
		targetCol := configString(cfg, "target_column_id")

		parentIDs := asStringSlice(nil)
		if childLink != "" && childData != nil {
			parentIDs = asStringSlice(childData[childLink])
		}
		for _, parentID := range parentIDs {
			val, err := computeRollupValue(ctx, pool, tables, tenantID, childVT, childLink, parentID, agg, targetCol)
			if err != nil {
				continue
			}
			key := "_rollup_" + colName
			patch, _ := json.Marshal(map[string]any{key: val})
			_, _ = pool.Exec(ctx, fmt.Sprintf(`
				UPDATE %s SET data = data || $1::jsonb, updated_at = now()
				WHERE vt_id = $2 AND tenant_id = $3 AND record_id = $4`,
				tables.QRecord(),
			), patch, parentVT, tenantID, parentID)
		}
	}
	return rows.Err()
}

func asStringSlice(v any) []string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		var out []string
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case []string:
		return t
	default:
		if v == nil {
			return nil
		}
		s := fmt.Sprint(v)
		if s == "" || s == "<nil>" {
			return nil
		}
		return []string{s}
	}
}

func computeRollupValue(ctx context.Context, pool *pgxpool.Pool, tables postgres.DataTables, tenantID, childVT, linkCol, parentID, agg, targetCol string) (any, error) {
	tbl := tables.QRecord()
	linkPred := fmt.Sprintf(`data->%s @> $3::jsonb`, quoteJSONKey(linkCol))
	parentArr, _ := json.Marshal([]string{parentID})

	var q string
	switch agg {
	case "count":
		q = fmt.Sprintf(`SELECT COUNT(*)::float8 FROM %s WHERE vt_id = $1 AND tenant_id = $2 AND %s`, tbl, linkPred)
	case "sum":
		q = fmt.Sprintf(`SELECT COALESCE(SUM((data->>%s)::numeric),0)::float8 FROM %s WHERE vt_id = $1 AND tenant_id = $2 AND %s`,
			quoteJSONKey(targetCol), tbl, linkPred)
	case "min":
		q = fmt.Sprintf(`SELECT MIN((data->>%s)::numeric)::float8 FROM %s WHERE vt_id = $1 AND tenant_id = $2 AND %s`,
			quoteJSONKey(targetCol), tbl, linkPred)
	case "max":
		q = fmt.Sprintf(`SELECT MAX((data->>%s)::numeric)::float8 FROM %s WHERE vt_id = $1 AND tenant_id = $2 AND %s`,
			quoteJSONKey(targetCol), tbl, linkPred)
	case "avg":
		q = fmt.Sprintf(`SELECT COALESCE(AVG((data->>%s)::numeric),0)::float8 FROM %s WHERE vt_id = $1 AND tenant_id = $2 AND %s`,
			quoteJSONKey(targetCol), tbl, linkPred)
	default:
		return nil, fmt.Errorf("unsupported aggregate %q", agg)
	}
	var v *float64
	if err := pool.QueryRow(ctx, q, childVT, tenantID, parentArr).Scan(&v); err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return *v, nil
}

func quoteJSONKey(k string) string {
	return "'" + strings.ReplaceAll(k, "'", "''") + "'"
}
