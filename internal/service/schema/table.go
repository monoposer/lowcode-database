package schema

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apiv1schema "github.com/monoposer/lowcode-database/internal/apiv1/schema"
	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"time"
)

func (s *Schema) CreateTable(ctx context.Context, req *apiv1schema.CreateTableRequest) (*apiv1schema.CreateTableResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if err := shared.ValidateTableName(req.Name); err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	schemaName := req.SchemaName
	if schemaName == "" {
		schemaName = "public"
	}
	idType, err := resolveTableIDTypeID(req.IdType)
	if err != nil {
		return nil, err
	}
	idDDL, err := buildTableSystemColumnsDDL(idType)
	if err != nil {
		return nil, err
	}

	dtx, err := data.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer dtx.Rollback(ctx)

	if _, err := dtx.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, pgx.Identifier{schemaName}.Sanitize())); err != nil {
		return nil, err
	}

	createSQL := fmt.Sprintf(`CREATE TABLE %s.%s (%s)`,
		pgx.Identifier{schemaName}.Sanitize(), pgx.Identifier{req.Name}.Sanitize(), idDDL)
	if _, err := dtx.Exec(ctx, createSQL); err != nil {
		return nil, err
	}
	if _, err := loadPhysicalIDPgType(ctx, dtx, schemaName, req.Name); err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}

	if err := dtx.Commit(ctx); err != nil {
		return nil, err
	}

	const ins = `
		INSERT INTO lc_tables (tenant_id, name, label, schema_name)
		VALUES ($1, $2, $3, $4)
		RETURNING name, label, schema_name, created_at, updated_at
	`
	row := meta.QueryRow(ctx, ins, tid, req.Name, req.Label, schemaName)

	var t apiv1schema.Table
	var createdAt, updatedAt time.Time
	if err := row.Scan(&t.Name, &t.Label, &t.SchemaName, &createdAt, &updatedAt); err != nil {
		dropSQL := fmt.Sprintf(`DROP TABLE IF EXISTS %s.%s`,
			pgx.Identifier{schemaName}.Sanitize(), pgx.Identifier{req.Name}.Sanitize())
		_, _ = data.Exec(ctx, dropSQL)
		return nil, err
	}
	t.Id = t.Name
	t.CreatedAt = createdAt
	t.UpdatedAt = updatedAt
	t.IdType = idType
	if err := s.fillTableIDType(ctx, &t); err != nil {
		return nil, err
	}
	if err := s.registerSystemColumns(ctx, meta, tid, req.Name, idType); err != nil {
		return nil, err
	}
	s.B.EmitEvent(ctx, event.MetadataTableCreated, t.Name, map[string]any{
		"table": tableToMap(&t),
	})

	return &apiv1schema.CreateTableResponse{Table: &t}, nil
}

func (s *Schema) registerSystemColumns(ctx context.Context, meta *pgxpool.Pool, tid, tableName, idType string) error {
	const ins = `
		INSERT INTO lc_columns (tenant_id, table_id, name, label, type_id, is_nullable, position, config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, '{"system":true}'::jsonb)
	`
	if _, err := meta.Exec(ctx, ins, tid, tableName, "id", "ID", idType, false, 0); err != nil {
		return err
	}
	_, err := meta.Exec(ctx, ins, tid, tableName, "updated_at", "Updated At", "timestamptz", false, 1)
	return err
}

func tableToMap(t *apiv1schema.Table) map[string]any {
	if t == nil {
		return nil
	}
	return map[string]any{
		"id": t.Id, "name": t.Name, "label": t.Label,
		"schemaName": t.SchemaName, "idType": t.IdType,
	}
}

func columnToMap(c *apiv1schema.Column) map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{
		"id": c.Id, "tableId": c.TableId, "name": c.Name,
		"typeId": c.TypeId, "label": c.Label,
	}
}

func (s *Schema) ListTables(ctx context.Context, _ *apiv1schema.ListTablesRequest) (*apiv1schema.ListTablesResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	const q = `SELECT name, label, schema_name, created_at, updated_at FROM lc_tables WHERE tenant_id = $1 ORDER BY created_at`
	rows, err := meta.Query(ctx, q, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res apiv1schema.ListTablesResponse
	for rows.Next() {
		var t apiv1schema.Table
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&t.Name, &t.Label, &t.SchemaName, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.Id = t.Name
		t.CreatedAt = createdAt
		t.UpdatedAt = updatedAt
		res.Tables = append(res.Tables, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.fillTableIDTypes(ctx, res.Tables); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *Schema) GetTableSchema(ctx context.Context, req *apiv1schema.GetTableSchemaRequest) (*apiv1schema.GetTableSchemaResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	if req.TableId == "" {
		return nil, fmt.Errorf("table_id is required")
	}

	var tbl apiv1schema.Table
	row := meta.QueryRow(ctx, `
		SELECT name, label, schema_name, created_at, updated_at
		FROM lc_tables
		WHERE name = $1 AND tenant_id = $2
	`, req.TableId, tid)
	var tblCreatedAt, tblUpdatedAt time.Time
	if err := row.Scan(&tbl.Name, &tbl.Label, &tbl.SchemaName, &tblCreatedAt, &tblUpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("table not found")
		}
		return nil, err
	}
	tbl.Id = tbl.Name
	tbl.CreatedAt = tblCreatedAt
	tbl.UpdatedAt = tblUpdatedAt
	if err := s.fillTableIDType(ctx, &tbl); err != nil {
		return nil, err
	}

	colRows, err := meta.Query(ctx, `
		SELECT id, table_id, name, label, type_id, is_nullable, position, config, created_at, updated_at
		FROM lc_columns
		WHERE table_id = $1 AND tenant_id = $2
		ORDER BY position
	`, req.TableId, tid)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	var columns []*apiv1schema.Column
	for colRows.Next() {
		var c apiv1schema.Column
		var cfg map[string]any
		var createdAt, updatedAt time.Time
		if err := colRows.Scan(&c.Id, &c.TableId, &c.Name, &c.Label, &c.TypeId, &c.IsNullable, &c.Position, &cfg, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt = createdAt
		c.UpdatedAt = updatedAt
		if cfg != nil {
			c.Config = cfg
		}
		PublicColumn(&c)
		columns = append(columns, &c)
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	indexes, err := listTableIndexesViaCatalog(s, ctx, req.TableId, tbl.SchemaName, tbl.Name)
	if err != nil {
		return nil, err
	}

	return &apiv1schema.GetTableSchemaResponse{
		Table:   &tbl,
		Columns: columns,
		Indexes: indexes,
	}, nil
}

func (s *Schema) DeleteTable(ctx context.Context, req *apiv1schema.DeleteTableRequest) (*apiv1schema.DeleteTableResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}

	var schemaName string
	if err := meta.QueryRow(ctx, `SELECT schema_name FROM lc_tables WHERE name = $1 AND tenant_id = $2`, req.Id, tid).
		Scan(&schemaName); err != nil {
		if err == pgx.ErrNoRows {
			return &apiv1schema.DeleteTableResponse{}, nil
		}
		return nil, err
	}

	dropSQL := fmt.Sprintf(`DROP TABLE IF EXISTS %s.%s`,
		pgx.Identifier{schemaName}.Sanitize(), pgx.Identifier{req.Id}.Sanitize())
	if _, err := data.Exec(ctx, dropSQL); err != nil {
		return nil, err
	}

	if _, err := meta.Exec(ctx, `DELETE FROM lc_tables WHERE name = $1 AND tenant_id = $2`, req.Id, tid); err != nil {
		return nil, err
	}
	s.B.InvalidateTableMetaCache(ctx, req.Id)
	s.B.EmitEvent(ctx, event.MetadataTableDeleted, req.Id, map[string]any{"tableId": req.Id})
	return &apiv1schema.DeleteTableResponse{}, nil
}

// RenameTable renames a table (meta logical name and PG physical table stay in sync).
func (s *Schema) RenameTable(ctx context.Context, req *apiv1schema.RenameTableRequest) (*apiv1schema.RenameTableResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	oldName := req.Id
	newName := req.NewName
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
	if err := meta.QueryRow(ctx, `SELECT 1 FROM lc_tables WHERE name = $1 AND tenant_id = $2`, newName, tid).Scan(&exists); err == nil {
		return nil, fmt.Errorf("table name %q already exists", newName)
	} else if err != pgx.ErrNoRows {
		return nil, err
	}

	var schemaName string
	if err := meta.QueryRow(ctx, `SELECT schema_name FROM lc_tables WHERE name = $1 AND tenant_id = $2`, oldName, tid).
		Scan(&schemaName); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("table not found")
		}
		return nil, err
	}

	renameSQL := fmt.Sprintf(`ALTER TABLE %s.%s RENAME TO %s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{oldName}.Sanitize(),
		pgx.Identifier{newName}.Sanitize(),
	)
	if _, err := data.Exec(ctx, renameSQL); err != nil {
		return nil, err
	}
	metaOK := false
	defer func() {
		if !metaOK {
			rev := fmt.Sprintf(`ALTER TABLE %s.%s RENAME TO %s`,
				pgx.Identifier{schemaName}.Sanitize(),
				pgx.Identifier{newName}.Sanitize(),
				pgx.Identifier{oldName}.Sanitize(),
			)
			_, _ = data.Exec(ctx, rev)
		}
	}()

	tx, err := meta.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := dropTableRenameFKConstraints(ctx, tx); err != nil {
		return nil, err
	}
	if err := updateTableRenameMetaRefs(ctx, tx, tid, oldName, newName); err != nil {
		return nil, err
	}
	if err := restoreTableRenameFKConstraints(ctx, tx); err != nil {
		return nil, err
	}

	const sel = `SELECT name, label, schema_name, created_at, updated_at FROM lc_tables WHERE name = $1 AND tenant_id = $2`
	row := tx.QueryRow(ctx, sel, newName, tid)
	var t apiv1schema.Table
	var createdAt, updatedAt time.Time
	if err := row.Scan(&t.Name, &t.Label, &t.SchemaName, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	t.Id = t.Name
	t.CreatedAt = createdAt
	t.UpdatedAt = updatedAt

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	metaOK = true
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
	return &apiv1schema.RenameTableResponse{Table: &t}, nil
}

func dropTableRenameFKConstraints(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		ALTER TABLE lc_columns DROP CONSTRAINT IF EXISTS lc_columns_tenant_table_fk;
		ALTER TABLE lc_relations DROP CONSTRAINT IF EXISTS lc_relations_tenant_id_source_table_id_fkey;
		ALTER TABLE lc_relations DROP CONSTRAINT IF EXISTS lc_relations_tenant_id_target_table_id_fkey;
		ALTER TABLE lc_data_sources DROP CONSTRAINT IF EXISTS lc_data_sources_tenant_id_table_id_fkey;
	`)
	return err
}

func updateTableRenameMetaRefs(ctx context.Context, tx pgx.Tx, tid, oldName, newName string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE lc_tables SET name = $1, updated_at = now() WHERE name = $2 AND tenant_id = $3
	`, newName, oldName, tid); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE lc_columns SET table_id = $1 WHERE table_id = $2 AND tenant_id = $3`, newName, oldName, tid); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_columns
		SET config = jsonb_set(config, '{target_table_id}', to_jsonb($1::text), true),
		    updated_at = now()
		WHERE config->>'target_table_id' = $2 AND tenant_id = $3
	`, newName, oldName, tid); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_relations SET source_table_id = $1, updated_at = now()
		WHERE source_table_id = $2 AND tenant_id = $3
	`, newName, oldName, tid); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_relations SET target_table_id = $1, updated_at = now()
		WHERE target_table_id = $2 AND tenant_id = $3
	`, newName, oldName, tid); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_data_sources SET table_id = $1, updated_at = now()
		WHERE table_id = $2 AND tenant_id = $3
	`, newName, oldName, tid); err != nil {
		return err
	}
	return nil
}

func restoreTableRenameFKConstraints(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		ALTER TABLE lc_columns
		  ADD CONSTRAINT lc_columns_tenant_table_fk
		  FOREIGN KEY (tenant_id, table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE;
		ALTER TABLE lc_relations
		  ADD CONSTRAINT lc_relations_tenant_id_source_table_id_fkey
		  FOREIGN KEY (tenant_id, source_table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE;
		ALTER TABLE lc_relations
		  ADD CONSTRAINT lc_relations_tenant_id_target_table_id_fkey
		  FOREIGN KEY (tenant_id, target_table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE;
		ALTER TABLE lc_data_sources
		  ADD CONSTRAINT lc_data_sources_tenant_id_table_id_fkey
		  FOREIGN KEY (tenant_id, table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE;
	`)
	return err
}
