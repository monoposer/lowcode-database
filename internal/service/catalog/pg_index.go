package catalog

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"regexp"
	"strings"
)

type pgIndexRow struct {
	Name      string
	IsUnique  bool
	PgColumns []string
}

func (s *Catalog) ListPGIndexes(ctx context.Context, schemaName, tableName string) ([]pgIndexRow, error) {
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT
			ic.relname AS index_name,
			idx.indisunique,
			COALESCE(array_agg(a.attname ORDER BY k.ord) FILTER (WHERE a.attname IS NOT NULL), '{}') AS columns
		FROM pg_class tbl
		JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
		JOIN pg_index idx ON idx.indrelid = tbl.oid
		JOIN pg_class ic ON ic.oid = idx.indexrelid
		LEFT JOIN LATERAL unnest(idx.indkey) WITH ORDINALITY AS k(attnum, ord) ON true
		LEFT JOIN pg_attribute a ON a.attrelid = tbl.oid AND a.attnum = k.attnum AND a.attnum > 0
		WHERE ns.nspname = $1
		  AND tbl.relname = $2
		  AND NOT idx.indisprimary
		GROUP BY ic.relname, idx.indisunique
		ORDER BY ic.relname
	`
	rows, err := data.Query(ctx, q, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pgIndexRow
	for rows.Next() {
		var r pgIndexRow
		if err := rows.Scan(&r.Name, &r.IsUnique, &r.PgColumns); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Catalog) PGIndexesToAPI(ctx context.Context, tableID, schemaName, tableName string, rows []pgIndexRow) ([]*apiv1schema.Index, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	pgToID := map[string]string{}
	idRows, err := meta.Query(ctx, `
		SELECT id, name FROM lc_columns WHERE table_id = $1 AND tenant_id = $2`, tableID, tid)
	if err != nil {
		return nil, err
	}
	defer idRows.Close()
	for idRows.Next() {
		var id, colName string
		if err := idRows.Scan(&id, &colName); err != nil {
			return nil, err
		}
		pgToID[colName] = id
	}
	if err := idRows.Err(); err != nil {
		return nil, err
	}

	var indexes []*apiv1schema.Index
	for _, r := range rows {
		idx := &apiv1schema.Index{
			Id:        r.Name,
			TableId:   tableID,
			Name:      r.Name,
			PgIndex:   r.Name,
			IsUnique:  r.IsUnique,
			ColumnIds: []string{},
		}
		for _, pgCol := range r.PgColumns {
			if id, ok := pgToID[pgCol]; ok {
				idx.ColumnIds = append(idx.ColumnIds, id)
			}
		}
		indexes = append(indexes, idx)
	}
	return indexes, nil
}

func (s *Catalog) resolveIndexSchema(ctx context.Context, indexName string) (string, error) {
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return "", err
	}
	var schema string
	if err := data.QueryRow(ctx, `
		SELECT schemaname FROM pg_indexes WHERE indexname = $1 LIMIT 1`, indexName).Scan(&schema); err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("index not found")
		}
		return "", err
	}
	return schema, nil
}

func indexSQLName(tableName, logicalName string) (string, error) {
	if logicalName == "" {
		return "", fmt.Errorf("index name is required")
	}
	n, err := sanitizePgIdent(logicalName)
	if err != nil {
		return "", err
	}
	tbl, err := sanitizePgIdent(tableName)
	if err != nil {
		tbl = "tbl"
	}
	return "idx_" + tbl + "_" + n, nil
}

var pgIdentRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

func sanitizePgIdent(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "_")
	if !pgIdentRe.MatchString(s) {
		return "", fmt.Errorf("invalid identifier %q (use lowercase letters, digits, underscore)", s)
	}
	return s, nil
}
