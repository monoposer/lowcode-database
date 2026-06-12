package schema

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/solat/lowcode-database/internal/service/shared"
)

func (s *Schema) NormalizeRelationshipConfig(ctx context.Context, tenantID, sourceTableKey string, cfg map[string]any) (map[string]any, error) {
	out, err := shared.NormalizeRelationshipConfig(cfg)
	if err != nil {
		return nil, err
	}
	targetTable, err := s.B.ResolveTableName(ctx, shared.CfgString(out, "target_table_id"))
	if err != nil {
		return nil, fmt.Errorf("relationship target_table_id: %w", err)
	}
	out["target_table_id"] = targetTable
	if link := shared.CfgString(out, "link_column_id"); link != "" {
		name, err := s.ResolveColumnName(ctx, tenantID, targetTable, link)
		if err != nil {
			return nil, fmt.Errorf("relationship link_column_id: %w", err)
		}
		out["link_column_id"] = name
	}
	if tgt := shared.CfgString(out, "target_column_id"); tgt != "" {
		name, err := s.ResolveColumnName(ctx, tenantID, sourceTableKey, tgt)
		if err != nil {
			return nil, fmt.Errorf("relationship target_column_id: %w", err)
		}
		out["target_column_id"] = name
	}
	return out, nil
}

func (s *Schema) NormalizeRelationFKConfig(ctx context.Context, tenantID string, cfg map[string]any) (map[string]any, error) {
	out := mapsClone(cfg)
	targetTable, err := s.B.ResolveTableName(ctx, shared.CfgString(out, "target_table_id"))
	if err != nil {
		return nil, fmt.Errorf("relation_fk target_table_id: %w", err)
	}
	out["target_table_id"] = targetTable
	if ref := shared.CfgString(out, "target_column_id"); ref != "" {
		name, err := s.ResolveColumnName(ctx, tenantID, targetTable, ref)
		if err != nil {
			return nil, fmt.Errorf("relation_fk target_column_id: %w", err)
		}
		out["target_column_id"] = name
	}
	return out, nil
}

func (s *Schema) NormalizeLookupConfig(ctx context.Context, tenantID, sourceTableKey string, cfg map[string]any) (map[string]any, error) {
	if cfg == nil {
		cfg = map[string]any{}
	}
	out := mapsClone(cfg)
	relRef := shared.CfgString(out, "relation_column_id")
	fieldRef := shared.CfgString(out, "target_column_id")
	if relRef == "" || fieldRef == "" {
		return nil, fmt.Errorf("lookup config requires relation_column_id and target_column_id")
	}
	relName, err := s.ResolveColumnName(ctx, tenantID, sourceTableKey, relRef)
	if err != nil {
		return nil, fmt.Errorf("lookup relation_column_id: %w", err)
	}
	out["relation_column_id"] = relName

	meta := s.B.Tenants.MetaPool()
	var relCfg map[string]any
	err = meta.QueryRow(ctx, `
		SELECT config FROM lc_columns
		WHERE tenant_id = $1 AND table_id = $2 AND name = $3 AND type_id = 'relationship'`,
		tenantID, sourceTableKey, relName,
	).Scan(&relCfg)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("lookup relation_column_id must reference a relationship column on the same table")
	}
	if err != nil {
		return nil, err
	}
	normRel, err := shared.NormalizeRelationshipConfig(relCfg)
	if err != nil {
		return nil, fmt.Errorf("lookup: invalid relationship config: %w", err)
	}
	targetTable, err := s.B.ResolveTableName(ctx, shared.CfgString(normRel, "target_table_id"))
	if err != nil {
		return nil, err
	}
	fieldName, err := s.ResolveColumnName(ctx, tenantID, targetTable, fieldRef)
	if err != nil {
		return nil, fmt.Errorf("lookup target_column_id: %w", err)
	}
	out["target_column_id"] = fieldName
	if err := shared.ValidateLinkedFilter(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Schema) NormalizeRollupConfig(ctx context.Context, tenantID, sourceTableKey string, cfg map[string]any) (map[string]any, error) {
	if err := shared.ValidateRollupConfig(cfg); err != nil {
		return nil, err
	}
	out := mapsClone(cfg)
	relName, err := s.ResolveColumnName(ctx, tenantID, sourceTableKey, shared.CfgString(out, "relation_column_id"))
	if err != nil {
		return nil, fmt.Errorf("rollup relation_column_id: %w", err)
	}
	out["relation_column_id"] = relName
	if fieldRef := shared.CfgString(out, "target_column_id"); fieldRef != "" {
		meta := s.B.Tenants.MetaPool()
		var relCfg map[string]any
		if err := meta.QueryRow(ctx, `
			SELECT config FROM lc_columns
			WHERE tenant_id = $1 AND table_id = $2 AND name = $3 AND type_id = 'relationship'`,
			tenantID, sourceTableKey, relName,
		).Scan(&relCfg); err != nil {
			return nil, fmt.Errorf("rollup relation_column_id: relationship column not found")
		}
		targetTable, err := s.B.ResolveTableName(ctx, shared.CfgString(relCfg, "target_table_id"))
		if err != nil {
			return nil, err
		}
		fieldName, err := s.ResolveColumnName(ctx, tenantID, targetTable, fieldRef)
		if err != nil {
			return nil, fmt.Errorf("rollup target_column_id: %w", err)
		}
		out["target_column_id"] = fieldName
	}
	return out, nil
}

func mapsClone(cfg map[string]any) map[string]any {
	if cfg == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(cfg))
	for k, v := range cfg {
		out[k] = v
	}
	return out
}

// ResolveColumnName accepts a column logical name or meta UUID within tableKey
// and returns the canonical logical name (same convention as Table.Id = name).
func (s *Schema) ResolveColumnName(ctx context.Context, tenantID, tableKey, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("column ref is required")
	}
	resolvedTable, err := s.B.ResolveTableName(ctx, tableKey)
	if err != nil {
		return "", err
	}
	meta := s.B.Tenants.MetaPool()
	var name string
	err = meta.QueryRow(ctx, `
		SELECT name FROM lc_columns
		WHERE tenant_id = $1 AND table_id = $2 AND name = $3`,
		tenantID, resolvedTable, ref,
	).Scan(&name)
	if err == nil {
		return name, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	if _, parseErr := uuid.Parse(ref); parseErr != nil {
		return "", fmt.Errorf("column %q not found on table %q", ref, resolvedTable)
	}
	err = meta.QueryRow(ctx, `
		SELECT name FROM lc_columns
		WHERE tenant_id = $1 AND table_id = $2 AND id = $3::uuid`,
		tenantID, resolvedTable, ref,
	).Scan(&name)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("column %q not found on table %q", ref, resolvedTable)
	}
	if err != nil {
		return "", err
	}
	return name, nil
}

func (s *Schema) ResolveColumnUUID(ctx context.Context, tenantID, tableKey, ref string) (string, error) {
	name, err := s.ResolveColumnName(ctx, tenantID, tableKey, ref)
	if err != nil {
		return "", err
	}
	resolvedTable, err := s.B.ResolveTableName(ctx, tableKey)
	if err != nil {
		return "", err
	}
	var id string
	err = s.B.Tenants.MetaPool().QueryRow(ctx, `
		SELECT id::text FROM lc_columns
		WHERE tenant_id = $1 AND table_id = $2 AND name = $3`,
		tenantID, resolvedTable, name,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// ResolveColumnDBID resolves a column ref (logical name or meta UUID) to lc_columns.id.
// When ref is not a UUID, tableKey is required.
func (s *Schema) ResolveColumnDBID(ctx context.Context, tenantID, tableKey, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("column ref is required")
	}
	if _, parseErr := uuid.Parse(ref); parseErr == nil {
		var id string
		err := s.B.Tenants.MetaPool().QueryRow(ctx, `
			SELECT id::text FROM lc_columns WHERE id = $1::uuid AND tenant_id = $2`,
			ref, tenantID,
		).Scan(&id)
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("column not found")
		}
		return id, err
	}
	if tableKey == "" {
		return "", fmt.Errorf("table_id is required when column ref is a name")
	}
	return s.ResolveColumnUUID(ctx, tenantID, tableKey, ref)
}

func (s *Schema) ColumnPgColumnByRef(ctx context.Context, tenantID, tableKey, ref string) (string, error) {
	return s.ResolveColumnName(ctx, tenantID, tableKey, ref)
}

func (s *Schema) NormalizeColumnNames(ctx context.Context, tenantID, tableKey string, refs []string) ([]string, error) {
	out := make([]string, 0, len(refs))
	seen := map[string]struct{}{}
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		name, err := s.ResolveColumnName(ctx, tenantID, tableKey, ref)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}

func ColumnRefInSet(c shared.ColumnMeta, want map[string]struct{}) bool {
	if _, ok := want[c.Name]; ok {
		return true
	}
	if _, ok := want[c.Id]; ok {
		return true
	}
	return false
}

func ColumnRefMatches(meta shared.ColumnMeta, ref string) bool {
	if ref == "" {
		return false
	}
	return meta.Name == ref || meta.Id == ref
}

// LoadManyRelationshipColumns returns many-cardinality relationship columns keyed by column name.
func (s *Schema) LoadManyRelationshipColumns(ctx context.Context, tableID string) (map[string]shared.RelationshipColumn, error) {
	resolvedName, err := s.B.ResolveTableName(ctx, tableID)
	if err != nil {
		return nil, err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	const q = `
		SELECT c.name, c.config
		FROM lc_columns c
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.type_id = 'relationship'
	`
	rows, err := meta.Query(ctx, q, resolvedName, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]shared.RelationshipColumn)
	for rows.Next() {
		var name string
		var cfg map[string]any
		if err := rows.Scan(&name, &cfg); err != nil {
			return nil, err
		}
		linkID := shared.CfgString(cfg, "link_column_id")
		targetColID := shared.CfgString(cfg, "target_column_id")
		card := shared.EffectiveRelationshipCardinality(cfg, linkID, targetColID)
		if card != "many" {
			continue
		}
		targetTable := shared.CfgString(cfg, "target_table_id")
		if targetTable == "" || linkID == "" {
			continue
		}
		out[name] = shared.RelationshipColumn{
			Id:            name,
			TargetTableId: targetTable,
			LinkColumnId:  linkID,
			Cardinality:   "many",
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// LoadOneRelationshipColumns returns one-cardinality relationship columns keyed by column name.
func (s *Schema) LoadOneRelationshipColumns(ctx context.Context, tableID string) (map[string]shared.RelationshipColumn, error) {
	resolvedName, err := s.B.ResolveTableName(ctx, tableID)
	if err != nil {
		return nil, err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	const q = `
		SELECT c.name, c.config
		FROM lc_columns c
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.type_id = 'relationship'
	`
	rows, err := meta.Query(ctx, q, resolvedName, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]shared.RelationshipColumn)
	for rows.Next() {
		var name string
		var cfg map[string]any
		if err := rows.Scan(&name, &cfg); err != nil {
			return nil, err
		}
		linkID := shared.CfgString(cfg, "link_column_id")
		targetColID := shared.CfgString(cfg, "target_column_id")
		card := shared.EffectiveRelationshipCardinality(cfg, linkID, targetColID)
		if card != "one" {
			continue
		}
		targetTable := shared.CfgString(cfg, "target_table_id")
		if targetTable == "" || targetColID == "" {
			continue
		}
		out[name] = shared.RelationshipColumn{
			Id:             name,
			TargetTableId:  targetTable,
			TargetColumnId: targetColID,
			Cardinality:    "one",
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// MustFKColumnName resolves target_column_id to physical FK column name on the host table.
func (s *Schema) MustFKColumnName(ctx context.Context, tenantID, hostTableID, fkColumnRef string) (string, error) {
	name, err := s.ResolveColumnName(ctx, tenantID, hostTableID, fkColumnRef)
	if err != nil {
		return "", fmt.Errorf("fk column: %w", err)
	}
	return name, nil
}

// RelationshipColumnNamesSet returns a set of keys for ClassifySaveGraphFields.
func RelationshipColumnNamesSet(rels map[string]shared.RelationshipColumn) map[string]struct{} {
	set := make(map[string]struct{}, len(rels))
	for k := range rels {
		set[k] = struct{}{}
	}
	return set
}

// MustLinkColumnName resolves link_column_id to physical column name on child table.
func (s *Schema) MustLinkColumnName(ctx context.Context, tenantID, childTableID, linkColumnRef string) (string, error) {
	name, err := s.ResolveColumnName(ctx, tenantID, childTableID, linkColumnRef)
	if err != nil {
		return "", fmt.Errorf("link column: %w", err)
	}
	return name, nil
}
