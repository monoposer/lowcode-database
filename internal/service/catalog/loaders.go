package catalog

import (
	"context"

	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func (s *Catalog) LoadColumns(ctx context.Context, tableID string) ([]shared.ColumnMeta, string, string, error) {
	resolvedName, err := s.B.ResolveTableName(ctx, tableID)
	if err != nil {
		return nil, "", "", err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, "", "", err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, "", "", err
	}
	meta := s.B.Tenants.MetaPool()
	const q = `
		SELECT c.id, c.table_id, c.name, c.type_id, c.is_nullable, c.position, c.config,
		       t.name
		FROM lc_columns c
		JOIN lc_tables t ON c.table_id = t.name AND c.tenant_id = t.tenant_id AND c.base_id = t.base_id
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.base_id = $3
		ORDER BY c.position
	`
	rows, err := meta.Query(ctx, q, resolvedName, tid, baseID)
	if err != nil {
		return nil, "", "", err
	}
	defer rows.Close()

	var cols []shared.ColumnMeta
	var tableName string
	for rows.Next() {
		var c shared.ColumnMeta
		var cfg map[string]any
		if err := rows.Scan(&c.Id, &c.TableId, &c.Name, &c.TypeId, &c.IsNullable, &c.Position, &cfg,
			&tableName); err != nil {
			return nil, "", "", err
		}
		if columntype.IsVirtual(c.TypeId) {
			continue
		}
		c.PgType = s.ColumnPgTypeSQL(ctx, tid, c.TypeId, cfg)
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", "", err
	}
	return cols, "", tableName, nil
}

func (s *Catalog) LoadAllColumnMeta(ctx context.Context, tableID string) ([]shared.FullColumnMeta, string, string, error) {
	resolvedName, err := s.B.ResolveTableName(ctx, tableID)
	if err != nil {
		return nil, "", "", err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, "", "", err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, "", "", err
	}
	key := shared.CacheKeyColumns(tid, resolvedName)
	if s.B.Cache != nil {
		var cached shared.CachedColumnMetaBundle
		if ok, _ := s.B.Cache.Get(ctx, key, &cached); ok {
			return cached.Cols, cached.SchemaName, cached.TableName, nil
		}
	}

	meta := s.B.Tenants.MetaPool()
	const q = `
		SELECT c.id, c.table_id, c.name, c.type_id, c.is_nullable, c.position,
		       c.config, t.name
		FROM lc_columns c
		JOIN lc_tables t ON c.table_id = t.name AND c.tenant_id = t.tenant_id AND c.base_id = t.base_id
		WHERE c.table_id = $1 AND c.tenant_id = $2 AND c.base_id = $3
		ORDER BY c.position
	`
	rows, err := meta.Query(ctx, q, resolvedName, tid, baseID)
	if err != nil {
		return nil, "", "", err
	}
	defer rows.Close()

	var out []shared.FullColumnMeta
	var tableName string
	for rows.Next() {
		var c shared.FullColumnMeta
		if err := rows.Scan(&c.Id, &c.TableId, &c.Name, &c.TypeId, &c.IsNullable, &c.Position,
			&c.Config, &tableName); err != nil {
			return nil, "", "", err
		}
		c.PgType = s.ColumnPgTypeSQL(ctx, tid, c.TypeId, c.Config)
		c.Kind = columntype.Kind(c.TypeId)
		c.IsVirtual = columntype.IsVirtual(c.TypeId)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", "", err
	}
	if s.B.Cache != nil {
		_ = s.B.Cache.Set(ctx, key, shared.CachedColumnMetaBundle{
			Cols: out, SchemaName: "", TableName: tableName,
		}, s.B.CacheTTL)
	}
	return out, "", tableName, nil
}
