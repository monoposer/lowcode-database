package catalog

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"strings"
)

func (s *Catalog) findPgEnumSchema(ctx context.Context, data *pgxpool.Pool, typeName string) (string, error) {
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

func (s *Catalog) listTenantChoiceEnums(ctx context.Context, data *pgxpool.Pool, tenantID string) ([]struct{ Schema, TypeName string }, error) {
	legacyPrefix, err := legacyChoiceTypePrefix(tenantID)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT n.nspname, t.typname
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE t.typtype = 'e' AND n.nspname = 'public'
		  AND (
		    t.typname LIKE $1
		    OR (t.typname ~ '^[a-z][a-z0-9_]{0,62}$' AND t.typname NOT LIKE 'lc_e_%')
		  )
		ORDER BY t.typname
	`
	rows, err := data.Query(ctx, q, legacyPrefix+"%")
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

func (s *Catalog) createPgEnumType(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string, literals []string) error {
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

func (s *Catalog) addPgEnumValues(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string, literals []string) error {
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

func (s *Catalog) dropPgEnumType(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string) error {
	stmt := fmt.Sprintf(`DROP TYPE IF EXISTS %s.%s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{typeName}.Sanitize(),
	)
	_, err := data.Exec(ctx, stmt)
	return err
}

func (s *Catalog) replacePgEnumValues(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string, literals []string) error {
	if len(literals) == 0 {
		return fmt.Errorf("at least one enum value is required")
	}
	current, err := s.listPgEnumValues(ctx, data, schemaName, typeName)
	if err != nil {
		return err
	}
	oldLabels := make([]string, 0, len(current))
	for _, it := range current {
		oldLabels = append(oldLabels, it.Value)
	}
	newLabels := enumLabelsFromLiterals(literals)
	if !enumLabelsNeedRecreate(oldLabels, newLabels) {
		toAdd := enumLiteralsToAdd(oldLabels, literals)
		if len(toAdd) == 0 {
			return nil
		}
		return s.addPgEnumValues(ctx, data, schemaName, typeName, toAdd)
	}
	return s.recreatePgEnumType(ctx, data, schemaName, typeName, literals)
}

func (s *Catalog) recreatePgEnumType(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string, literals []string) error {
	refs, err := s.listPgEnumColumnRefs(ctx, data, schemaName, typeName)
	if err != nil {
		return err
	}
	tx, err := data.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if len(refs) == 0 {
		drop := fmt.Sprintf(`DROP TYPE %s.%s`,
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{typeName}.Sanitize(),
		)
		if _, err := tx.Exec(ctx, drop); err != nil {
			return err
		}
		create := fmt.Sprintf(`CREATE TYPE %s.%s AS ENUM (%s)`,
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{typeName}.Sanitize(),
			strings.Join(literals, ", "),
		)
		_, err = tx.Exec(ctx, create)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	tempName := typeName + "__lc_new"
	createTemp := fmt.Sprintf(`CREATE TYPE %s.%s AS ENUM (%s)`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tempName}.Sanitize(),
		strings.Join(literals, ", "),
	)
	if _, err := tx.Exec(ctx, createTemp); err != nil {
		return err
	}
	for _, ref := range refs {
		alter := fmt.Sprintf(
			`ALTER TABLE %s.%s ALTER COLUMN %s TYPE %s.%s USING %s::text::%s.%s`,
			pgx.Identifier{ref.Schema}.Sanitize(),
			pgx.Identifier{ref.TableName}.Sanitize(),
			pgx.Identifier{ref.ColumnName}.Sanitize(),
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{tempName}.Sanitize(),
			pgx.Identifier{ref.ColumnName}.Sanitize(),
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{tempName}.Sanitize(),
		)
		if _, err := tx.Exec(ctx, alter); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "invalid input value for enum") {
				return fmt.Errorf("cannot remove enum value: existing rows still use a removed label")
			}
			return err
		}
	}
	dropOld := fmt.Sprintf(`DROP TYPE %s.%s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{typeName}.Sanitize(),
	)
	if _, err := tx.Exec(ctx, dropOld); err != nil {
		return err
	}
	rename := fmt.Sprintf(`ALTER TYPE %s.%s RENAME TO %s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tempName}.Sanitize(),
		pgx.Identifier{typeName}.Sanitize(),
	)
	if _, err := tx.Exec(ctx, rename); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// choicePgTypeName returns the PostgreSQL ENUM type name (same as logical name; tenant DB is isolated).
func choicePgTypeName(_tenantID, choiceName string) (string, error) {
	return sanitizePgIdent(choiceName)
}

func legacyChoicePgTypeName(tenantID, choiceName string) (string, error) {
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

func legacyChoiceTypePrefix(tenantID string) (string, error) {
	t, err := sanitizePgIdent(tenantID)
	if err != nil {
		return "", fmt.Errorf("tenant id: %w", err)
	}
	return "lc_e_" + t + "_", nil
}

func choiceLogicalNameFromPgType(tenantID, pgTypeName string) (string, error) {
	if prefix, err := legacyChoiceTypePrefix(tenantID); err == nil {
		if strings.HasPrefix(pgTypeName, prefix) {
			name := strings.TrimPrefix(pgTypeName, prefix)
			if name != "" {
				return name, nil
			}
		}
	}
	n, err := sanitizePgIdent(pgTypeName)
	if err != nil {
		return "", fmt.Errorf("invalid choice pg type name %q", pgTypeName)
	}
	return n, nil
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

func enumValuesFromItems(items []*apiv1schema.ChoiceItem) ([]string, error) {
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

func enumLabelsFromLiterals(literals []string) []string {
	out := make([]string, 0, len(literals))
	for _, lit := range literals {
		out = append(out, strings.Trim(lit, `'`))
	}
	return out
}

func enumLabelSet(labels []string) map[string]struct{} {
	set := make(map[string]struct{}, len(labels))
	for _, l := range labels {
		set[l] = struct{}{}
	}
	return set
}

func enumLiteralsToAdd(oldLabels []string, literals []string) []string {
	oldSet := enumLabelSet(oldLabels)
	var add []string
	for _, lit := range literals {
		if _, ok := oldSet[strings.Trim(lit, `'`)]; !ok {
			add = append(add, lit)
		}
	}
	return add
}

func enumLabelsNeedRecreate(oldLabels, newLabels []string) bool {
	newSet := enumLabelSet(newLabels)
	for _, l := range oldLabels {
		if _, ok := newSet[l]; !ok {
			return true
		}
	}
	if len(oldLabels) == len(newLabels) {
		for i := range oldLabels {
			if oldLabels[i] != newLabels[i] {
				return true
			}
		}
		return false
	}
	return len(newLabels) < len(oldLabels)
}

type pgEnumColumnRef struct {
	Schema     string
	TableName  string
	ColumnName string
}

func (s *Catalog) listPgEnumColumnRefs(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string) ([]pgEnumColumnRef, error) {
	const q = `
		SELECT n.nspname, c.relname, a.attname
		FROM pg_type t
		JOIN pg_namespace tn ON tn.oid = t.typnamespace
		JOIN pg_attribute a ON a.atttypid = t.oid AND a.attnum > 0 AND NOT a.attisdropped
		JOIN pg_class c ON c.oid = a.attrelid AND c.relkind IN ('r', 'p')
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE tn.nspname = $1 AND t.typname = $2 AND t.typtype = 'e'
		ORDER BY n.nspname, c.relname, a.attname
	`
	rows, err := data.Query(ctx, q, schemaName, typeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pgEnumColumnRef
	for rows.Next() {
		var ref pgEnumColumnRef
		if err := rows.Scan(&ref.Schema, &ref.TableName, &ref.ColumnName); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

func (s *Catalog) listPgEnumValues(ctx context.Context, data *pgxpool.Pool, schemaName, typeName string) ([]*apiv1schema.ChoiceItem, error) {
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
	var items []*apiv1schema.ChoiceItem
	for rows.Next() {
		var label string
		var ord int32
		if err := rows.Scan(&label, &ord); err != nil {
			return nil, err
		}
		items = append(items, &apiv1schema.ChoiceItem{
			Name:  label,
			Label: label,
			Value: label,
			Order: ord,
		})
	}
	return items, rows.Err()
}

func (s *Catalog) ResolveChoicePgTypeRef(ctx context.Context, tid, ref string) (schemaName, typeName string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", fmt.Errorf("choice reference is required")
	}
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return "", "", err
	}
	logical, logicalErr := sanitizePgIdent(ref)
	if logicalErr == nil {
		if schema, err := s.findPgEnumSchema(ctx, data, logical); err == nil {
			return schema, logical, nil
		}
	}
	if strings.HasPrefix(strings.ToLower(ref), "lc_e_") {
		safe := strings.ToLower(ref)
		if schema, err := s.findPgEnumSchema(ctx, data, safe); err == nil {
			return schema, safe, nil
		}
	}
	if logicalErr == nil {
		if legacy, err := legacyChoicePgTypeName(tid, logical); err == nil {
			if schema, err := s.findPgEnumSchema(ctx, data, legacy); err == nil {
				return schema, legacy, nil
			}
		}
	}
	return "", "", fmt.Errorf("choice %q not found", ref)
}
