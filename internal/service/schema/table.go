package schema

import (
	"context"
	"fmt"
	"time"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
)

func (s *Schema) CreateTable(ctx context.Context, in *Table) (*Table, error) {
	if in == nil {
		return nil, fmt.Errorf("table is required")
	}
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	if err := shared.ValidateTableName(in.Name); err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	idType, err := resolveTableIDTypeID(in.IdType)
	if err != nil {
		return nil, err
	}

	ctx, tables, err := s.B.Tenants.AttachDataTables(ctx)
	if err != nil {
		return nil, err
	}
	if err := postgres.EnsureDataTables(ctx, data, tables); err != nil {
		return nil, fmt.Errorf("ensure virtual_records: %w", err)
	}
	vtID := uuid.NewString()
	if err := postgres.EnsureVirtualRecordsPartitionOn(ctx, data, tables, vtID); err != nil {
		return nil, fmt.Errorf("create partition: %w", err)
	}

	var t Table
	t.BaseId = baseID
	t.IdType = idType
	if err := meta.QueryRow(ctx, `
		INSERT INTO lc_tables (tenant_id, base_id, name, label, vt_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING name, label, created_at, updated_at`,
		tenantID, baseID, in.Name, in.Label, vtID,
	).Scan(&t.Name, &t.Label, &t.CreatedAt, &t.UpdatedAt); err != nil {
		_ = postgres.DropVirtualRecordsPartition(ctx, data, vtID)
		return nil, err
	}
	t.Id = t.Name
	if err := s.fillTableIDType(ctx, &t); err != nil {
		return nil, err
	}
	if err := s.registerSystemColumns(ctx, meta, tenantID, baseID, in.Name, idType); err != nil {
		return nil, err
	}
	s.B.EmitEvent(ctx, event.MetadataTableCreated, t.Name, map[string]any{
		"table": tableToMap(&t),
	})

	return &t, nil
}

func (s *Schema) registerSystemColumns(ctx context.Context, meta *pgxpool.Pool, tenantID, baseID, tableName, idType string) error {
	sys := map[string]any{"system": true}
	const ins = `
		INSERT INTO lc_columns (tenant_id, base_id, table_name, name, label, type_id, is_nullable, position, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := meta.Exec(ctx, ins, tenantID, baseID, tableName, "id", "ID", idType, false, 0, sys); err != nil {
		return err
	}
	_, err := meta.Exec(ctx, ins, tenantID, baseID, tableName, "updated_at", "Updated At", "datetime", false, 1, sys)
	return err
}

func tableToMap(t *Table) map[string]any {
	if t == nil {
		return nil
	}
	return map[string]any{
		"id": t.Id, "name": t.Name, "label": t.Label,
		"baseId": t.BaseId, "idType": t.IdType,
	}
}

func columnToMap(c *Column) map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{
		"id": c.Id, "tableName": c.TableName, "name": c.Name,
		"typeId": c.TypeId, "label": c.Label,
	}
}

func (s *Schema) ListTables(ctx context.Context) ([]*Table, error) {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	clause, args, _ := postgres.Where(1, postgres.TenantBase(tenantID, baseID)...)
	q := `SELECT name, label, created_at, updated_at FROM lc_tables WHERE ` + clause + ` ORDER BY created_at`
	rows, err := s.B.Tenants.MetaPool().Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []*Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Name, &t.Label, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Id = t.Name
		t.BaseId = baseID
		tables = append(tables, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.fillTableIDTypes(ctx, tables); err != nil {
		return nil, err
	}
	return tables, nil
}

func (s *Schema) GetTableSchema(ctx context.Context, tableName string) (*Table, []*Column, []*catalog.Index, error) {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	meta := s.B.Tenants.MetaPool()
	if tableName == "" {
		return nil, nil, nil, fmt.Errorf("table_name is required")
	}

	tblSQL := `SELECT name, label, created_at, updated_at FROM lc_tables WHERE name = $1`
	tblSQL, tblArgs, _ := postgres.AndWhere(tblSQL, []any{tableName}, 2, postgres.TenantBase(tenantID, baseID)...)
	var tbl Table
	if err := meta.QueryRow(ctx, tblSQL, tblArgs...).Scan(&tbl.Name, &tbl.Label, &tbl.CreatedAt, &tbl.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil, fmt.Errorf("table not found")
		}
		return nil, nil, nil, err
	}
	tbl.Id = tbl.Name
	tbl.BaseId = baseID
	if err := s.fillTableIDType(ctx, &tbl); err != nil {
		return nil, nil, nil, err
	}

	colSQL := `SELECT id, table_name, name, label, type_id, is_nullable, position, config, created_at, updated_at
		FROM lc_columns WHERE table_name = $1`
	colSQL, colArgs, _ := postgres.AndWhere(colSQL, []any{tableName}, 2, postgres.TenantBase(tenantID, baseID)...)
	colSQL += ` ORDER BY position`
	colRows, err := meta.Query(ctx, colSQL, colArgs...)
	if err != nil {
		return nil, nil, nil, err
	}
	defer colRows.Close()

	var columns []*Column
	for colRows.Next() {
		var c Column
		var cfg map[string]any
		if err := colRows.Scan(&c.Id, &c.TableName, &c.Name, &c.Label, &c.TypeId, &c.IsNullable, &c.Position, &cfg, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, nil, nil, err
		}
		c.Config = cfg
		PublicColumn(&c)
		columns = append(columns, &c)
	}
	if err := colRows.Err(); err != nil {
		return nil, nil, nil, err
	}

	indexes, err := catalog.New(s.B).ListIndexes(ctx, tableName)
	if err != nil {
		return nil, nil, nil, err
	}

	return &tbl, columns, indexes, nil
}

func (s *Schema) DeleteTable(ctx context.Context, id string) error {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return err
	}
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return err
	}

	sel := `SELECT vt_id::text FROM lc_tables WHERE name = $1`
	sel, selArgs, _ := postgres.AndWhere(sel, []any{id}, 2, postgres.TenantBase(tenantID, baseID)...)
	var vtID *string
	if err := s.B.Tenants.MetaPool().QueryRow(ctx, sel, selArgs...).Scan(&vtID); err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	var ddl string
	if vtID != nil && *vtID != "" {
		var err error
		ddl, err = postgres.DropVirtualRecordsPartitionSQL(ctx, data, *vtID)
		if err != nil {
			return err
		}
	}

	del := `DELETE FROM lc_tables WHERE name = $1`
	del, delArgs, _ := postgres.AndWhere(del, []any{id}, 2, postgres.TenantBase(tenantID, baseID)...)
	if _, err := s.B.Tenants.MetaPool().Exec(ctx, del, delArgs...); err != nil {
		return err
	}
	s.B.InvalidateTableMetaCache(ctx, id)
	s.B.EmitEvent(ctx, event.MetadataTableDeleted, id, map[string]any{"tableName": id, "ddl": ddl})
	return nil
}

func (s *Schema) RenameTable(ctx context.Context, id, newName string) (*Table, error) {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	oldName := id
	if oldName == "" {
		return nil, fmt.Errorf("id is required")
	}
	if newName == "" {
		return nil, fmt.Errorf("new_name is required")
	}
	if oldName == newName {
		return nil, fmt.Errorf("new_name must differ from current name")
	}
	if err := shared.ValidateTableName(newName); err != nil {
		return nil, err
	}

	var exists int
	if err := meta.QueryRow(ctx, `
		SELECT 1 FROM lc_tables WHERE name = $1 AND tenant_id = $2 AND base_id = $3`,
		newName, tenantID, baseID).Scan(&exists); err == nil {
		return nil, fmt.Errorf("table name %q already exists", newName)
	} else if err != pgx.ErrNoRows {
		return nil, err
	}

	var found string
	if err := meta.QueryRow(ctx, `
		SELECT name FROM lc_tables WHERE name = $1 AND tenant_id = $2 AND base_id = $3`,
		oldName, tenantID, baseID).Scan(&found); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("table not found")
		}
		return nil, err
	}

	tx, err := meta.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := dropTableRenameFKConstraints(ctx, tx); err != nil {
		return nil, err
	}
	if err := updateTableRenameMetaRefs(ctx, tx, tenantID, baseID, oldName, newName); err != nil {
		return nil, err
	}
	if err := restoreTableRenameFKConstraints(ctx, tx); err != nil {
		return nil, err
	}

	const sel = `SELECT name, label, created_at, updated_at FROM lc_tables WHERE name = $1 AND tenant_id = $2 AND base_id = $3`
	row := tx.QueryRow(ctx, sel, newName, tenantID, baseID)
	var t Table
	var createdAt, updatedAt time.Time
	if err := row.Scan(&t.Name, &t.Label, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	t.Id = t.Name
	t.BaseId = baseID
	t.CreatedAt = createdAt
	t.UpdatedAt = updatedAt

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.B.InvalidateTableMetaCache(ctx, oldName)
	s.B.InvalidateTableMetaCache(ctx, newName)
	if err := s.fillTableIDType(ctx, &t); err != nil {
		return nil, err
	}
	s.B.EmitEvent(ctx, event.MetadataTableRenamed, newName, map[string]any{
		"oldName": oldName,
		"newName": newName,
		"table":   tableToMap(&t),
	})
	return &t, nil
}

func dropTableRenameFKConstraints(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		ALTER TABLE lc_columns DROP CONSTRAINT IF EXISTS lc_columns_tenant_id_base_id_table_name_fkey;
		ALTER TABLE lc_queries DROP CONSTRAINT IF EXISTS lc_queries_tenant_id_base_id_table_name_fkey;
		ALTER TABLE lc_indexes DROP CONSTRAINT IF EXISTS lc_indexes_tenant_id_base_id_table_name_fkey;
	`)
	return err
}

func updateTableRenameMetaRefs(ctx context.Context, tx pgx.Tx, tenantID, baseID, oldName, newName string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE lc_tables SET name = $1, updated_at = now()
		WHERE name = $2 AND tenant_id = $3 AND base_id = $4
	`, newName, oldName, tenantID, baseID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_columns SET table_name = $1
		WHERE table_name = $2 AND tenant_id = $3 AND base_id = $4`,
		newName, oldName, tenantID, baseID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_columns
		SET config = jsonb_set(config, '{target_table_name}', to_jsonb($1::text), true),
		    updated_at = now()
		WHERE config->>'target_table_name' = $2 AND tenant_id = $3 AND base_id = $4
	`, newName, oldName, tenantID, baseID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_queries SET table_name = $1, updated_at = now()
		WHERE table_name = $2 AND tenant_id = $3 AND base_id = $4
	`, newName, oldName, tenantID, baseID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_indexes SET table_name = $1, updated_at = now()
		WHERE table_name = $2 AND tenant_id = $3 AND base_id = $4
	`, newName, oldName, tenantID, baseID); err != nil {
		return err
	}
	return nil
}

func restoreTableRenameFKConstraints(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		ALTER TABLE lc_columns
		  ADD CONSTRAINT lc_columns_tenant_id_base_id_table_name_fkey
		  FOREIGN KEY (tenant_id, base_id, table_name) REFERENCES lc_tables(tenant_id, base_id, name) ON DELETE CASCADE;
		ALTER TABLE lc_queries
		  ADD CONSTRAINT lc_queries_tenant_id_base_id_table_name_fkey
		  FOREIGN KEY (tenant_id, base_id, table_name) REFERENCES lc_tables(tenant_id, base_id, name) ON DELETE CASCADE;
		ALTER TABLE lc_indexes
		  ADD CONSTRAINT lc_indexes_tenant_id_base_id_table_name_fkey
		  FOREIGN KEY (tenant_id, base_id, table_name) REFERENCES lc_tables(tenant_id, base_id, name) ON DELETE CASCADE;
	`)
	return err
}
