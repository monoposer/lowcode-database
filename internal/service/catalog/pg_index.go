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
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	idRows, err := meta.Query(ctx, `
		SELECT id, name FROM lc_columns WHERE table_id = $1 AND tenant_id = $2 AND base_id = $3`, tableID, tid, baseID)
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
	prefix := "idx_" + tableName + "_"
	for _, r := range rows {
		logicalName := r.Name
		if len(r.Name) > len(prefix) && r.Name[:len(prefix)] == prefix {
			logicalName = r.Name[len(prefix):]
		}
		idx := &apiv1schema.Index{
			Id:        logicalName,
			TableId:   tableID,
			Name:      logicalName,
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

func (s *Catalog) resolveIndexSchema(ctx context.Context, indexName, tableID string) (string, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return "", err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return "", err
	}
	if _, err := s.findIndexMeta(ctx, tid, baseID, tableID, indexName); err == nil {
		return "", nil
	}
	schemaName, _, err := s.resolveIndexInPGCatalog(ctx, indexName, tableID)
	return schemaName, err
}

// resolveIndexInPGCatalog locates an index in PG catalog scoped to the current tenant's schemas.
func (s *Catalog) resolveIndexInPGCatalog(ctx context.Context, indexName, tableID string) (schemaName, pgTable string, err error) {
	if indexName == "" {
		return "", "", fmt.Errorf("index id is required")
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return "", "", err
	}
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return "", "", err
	}

	if tableID != "" {
		_, schemaName, tableName, err := s.B.LoadTablePhysical(ctx, tableID)
		if err != nil {
			return "", "", err
		}
		var foundSchema, foundTable string
		err = data.QueryRow(ctx, `
			SELECT schemaname, tablename FROM pg_indexes
			WHERE indexname = $1 AND schemaname = $2 AND tablename = $3`,
			indexName, schemaName, tableName,
		).Scan(&foundSchema, &foundTable)
		if err == pgx.ErrNoRows {
			return "", "", fmt.Errorf("index not found")
		}
		if err != nil {
			return "", "", err
		}
		return foundSchema, foundTable, nil
	}

	rows, err := data.Query(ctx, `
		SELECT i.schemaname, i.tablename
		FROM pg_indexes i
		WHERE i.indexname = $1
		  AND i.tablename IN (
		    SELECT name FROM lc_tables WHERE tenant_id = $2
		  )`, indexName, tid)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	var matches [][2]string
	for rows.Next() {
		var sch, tbl string
		if err := rows.Scan(&sch, &tbl); err != nil {
			return "", "", err
		}
		matches = append(matches, [2]string{sch, tbl})
	}
	if err := rows.Err(); err != nil {
		return "", "", err
	}
	switch len(matches) {
	case 0:
		return "", "", fmt.Errorf("index not found")
	case 1:
		return matches[0][0], matches[0][1], nil
	default:
		return "", "", fmt.Errorf("index %q is ambiguous; pass table_id query parameter", indexName)
	}
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
