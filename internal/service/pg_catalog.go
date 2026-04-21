package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/solat/lowcode-database/internal/apiv1"
)

var pgIdentRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

func sanitizePgIdent(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "_")
	if !pgIdentRe.MatchString(s) {
		return "", fmt.Errorf("invalid identifier %q (use lowercase letters, digits, underscore)", s)
	}
	return s, nil
}

func choicePgTypeName(tenantID, choiceName string) (string, error) {
	t, err := sanitizePgIdent(tenantID)
	if err != nil {
		return "", fmt.Errorf("tenant id: %w", err)
	}
	n, err := sanitizePgIdent(choiceName)
	if err != nil {
		return "", err
	}
	return "lc_e_" + t + "_" + n, nil
}

func choiceTypePrefix(tenantID string) (string, error) {
	t, err := sanitizePgIdent(tenantID)
	if err != nil {
		return "", fmt.Errorf("tenant id: %w", err)
	}
	return "lc_e_" + t + "_", nil
}

func choiceLogicalNameFromPgType(tenantID, pgTypeName string) (string, error) {
	prefix, err := choiceTypePrefix(tenantID)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(pgTypeName, prefix) {
		return "", fmt.Errorf("pg type %q is not a choice enum for tenant %q", pgTypeName, tenantID)
	}
	name := strings.TrimPrefix(pgTypeName, prefix)
	if name == "" {
		return "", fmt.Errorf("invalid choice pg type name %q", pgTypeName)
	}
	return name, nil
}

func (s *LowcodeService) findPgEnumSchema(ctx context.Context, data *pgxpool.Pool, typeName string) (string, error) {
	const q = `
		SELECT n.nspname
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE t.typname = $1 AND t.typtype = 'e'
		ORDER BY CASE WHEN n.nspname = 'public' THEN 0 ELSE 1 END, n.nspname
		LIMIT 1
	`
	var schema string
	if err := data.QueryRow(ctx, q, typeName).Scan(&schema); err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("choice enum type %q not found", typeName)
		}
		return "", err
	}
	return schema, nil
}

func (s *LowcodeService) listTenantChoiceEnums(ctx context.Context, data *pgxpool.Pool, tenantID string) ([]struct{ Schema, TypeName string }, error) {
	prefix, err := choiceTypePrefix(tenantID)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT n.nspname, t.typname
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE t.typtype = 'e' AND t.typname LIKE $1
		ORDER BY t.typname
	`
	rows, err := data.Query(ctx, q, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct{ Schema, TypeName string }
	for rows.Next() {
		var item struct{ Schema, TypeName string }
		if err := rows.Scan(&item.Schema, &item.TypeName); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *LowcodeService) resolveChoicePgTypeRef(ctx context.Context, tid, ref string) (schemaName, typeName string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", fmt.Errorf("choice reference is required")
	}
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return "", "", err
	}
	if strings.HasPrefix(ref, "lc_e_") {
		safe, err := sanitizePgIdent(ref)
		if err != nil {
			return "", "", fmt.Errorf("invalid choice pg type name")
		}
		schema, err := s.findPgEnumSchema(ctx, data, safe)
		return schema, safe, err
	}
	pgType, err := choicePgTypeName(tid, ref)
	if err != nil {
		return "", "", err
	}
	schema, err := s.findPgEnumSchema(ctx, data, pgType)
	return schema, pgType, err
}

func choiceEnumLiteral(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", fmt.Errorf("enum value cannot be empty")
	}
	// PG ENUM labels are string literals in DDL: 'active', 'inactive'
	v = strings.ReplaceAll(v, `'`, `''`)
	return `'` + v + `'`, nil
}

func enumValuesFromItems(items []*apiv1.ChoiceItem) ([]string, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one enum value is required")
	}
	seen := map[string]struct{}{}
	var out []string
	for _, it := range items {
		raw := strings.TrimSpace(it.Value)
		if raw == "" {
			raw = strings.TrimSpace(it.Name)
		}
		if raw == "" {
			return nil, fmt.Errorf("choice item requires value or name")
		}
		lit, err := choiceEnumLiteral(raw)
		if err != nil {
			return nil, err
		}
		key := strings.Trim(lit, `'`)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, lit)
	}
	return out, nil
}

func (s *LowcodeService) createPgEnumType(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string, literals []string) error {
	if len(literals) == 0 {
		return fmt.Errorf("enum literals required")
	}
	stmt := fmt.Sprintf(`CREATE TYPE %s.%s AS ENUM (%s)`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{typeName}.Sanitize(),
		strings.Join(literals, ", "),
	)
	_, err := data.Exec(ctx, stmt)
	return err
}

func (s *LowcodeService) addPgEnumValues(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string, literals []string) error {
	for _, lit := range literals {
		stmt := fmt.Sprintf(`ALTER TYPE %s.%s ADD VALUE IF NOT EXISTS %s`,
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{typeName}.Sanitize(),
			lit,
		)
		if _, err := data.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *LowcodeService) dropPgEnumType(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string) error {
	stmt := fmt.Sprintf(`DROP TYPE IF EXISTS %s.%s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{typeName}.Sanitize(),
	)
	_, err := data.Exec(ctx, stmt)
	return err
}

func (s *LowcodeService) listPgEnumValues(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string) ([]*apiv1.ChoiceItem, error) {
	const q = `
		SELECT e.enumlabel, e.enumsortorder
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		JOIN pg_enum e ON e.enumtypid = t.oid
		WHERE n.nspname = $1 AND t.typname = $2
		ORDER BY e.enumsortorder
	`
	rows, err := data.Query(ctx, q, schemaName, typeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*apiv1.ChoiceItem
	for rows.Next() {
		var label string
		var ord int32
		if err := rows.Scan(&label, &ord); err != nil {
			return nil, err
		}
		items = append(items, &apiv1.ChoiceItem{
			Name:  label,
			Label: label,
			Value: label,
			Order: ord,
		})
	}
	return items, rows.Err()
}

type pgIndexRow struct {
	Name      string
	IsUnique  bool
	PgColumns []string
}

func (s *LowcodeService) listPGIndexes(ctx context.Context, schemaName, tableName string) ([]pgIndexRow, error) {
	data, err := s.tenants.DataPool(ctx)
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

func (s *LowcodeService) pgIndexesToAPI(ctx context.Context, tableID, schemaName, tableName string, rows []pgIndexRow) ([]*apiv1.Index, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	pgToID := map[string]string{}
	idRows, err := meta.Query(ctx, `
		SELECT id, pg_column FROM lc_columns WHERE table_id = $1 AND tenant_id = $2`, tableID, tid)
	if err != nil {
		return nil, err
	}
	defer idRows.Close()
	for idRows.Next() {
		var id, pgCol string
		if err := idRows.Scan(&id, &pgCol); err != nil {
			return nil, err
		}
		pgToID[pgCol] = id
	}
	if err := idRows.Err(); err != nil {
		return nil, err
	}

	var indexes []*apiv1.Index
	for _, r := range rows {
		idx := &apiv1.Index{
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

func (s *LowcodeService) resolveIndexSchema(ctx context.Context, indexName string) (string, error) {
	data, err := s.tenants.DataPool(ctx)
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
