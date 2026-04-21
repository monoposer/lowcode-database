package service

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
)

var logicalTableNameRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

// -------- Table --------

func (s *LowcodeService) CreateTable(ctx context.Context, req *apiv1.CreateTableRequest) (*apiv1.CreateTableResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	schemaName := req.SchemaName
	if schemaName == "" {
		schemaName = "public"
	}
	// 物理表名与逻辑表名一致；pgx.Identifier 负责转义。
	physTable := req.Name

	dtx, err := data.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer dtx.Rollback(ctx)

	// Ensure schema exists
	if _, err := dtx.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, pgx.Identifier{schemaName}.Sanitize())); err != nil {
		return nil, err
	}

	// Create physical table with id column
	createSQL := fmt.Sprintf(`CREATE TABLE %s.%s (id UUID PRIMARY KEY DEFAULT gen_random_uuid())`,
		pgx.Identifier{schemaName}.Sanitize(), pgx.Identifier{physTable}.Sanitize())
	if _, err := dtx.Exec(ctx, createSQL); err != nil {
		return nil, err
	}

	if err := dtx.Commit(ctx); err != nil {
		return nil, err
	}

	const ins = `
		INSERT INTO lc_tables (tenant_id, name, schema_name, table_name)
		VALUES ($1, $2, $3, $4)
		RETURNING name, schema_name, table_name, created_at, updated_at
	`
	row := meta.QueryRow(ctx, ins, tid, req.Name, schemaName, physTable)

	var t apiv1.Table
	var createdAt, updatedAt time.Time
	if err := row.Scan(&t.Name, &t.SchemaName, &t.TableName, &createdAt, &updatedAt); err != nil {
		dropSQL := fmt.Sprintf(`DROP TABLE IF EXISTS %s.%s`,
			pgx.Identifier{schemaName}.Sanitize(), pgx.Identifier{physTable}.Sanitize())
		_, _ = data.Exec(ctx, dropSQL)
		return nil, err
	}
	// 对外约定：Table.Id 使用逻辑 name。
	t.Id = t.Name
	t.CreatedAt = createdAt
	t.UpdatedAt = updatedAt

	return &apiv1.CreateTableResponse{Table: &t}, nil
}

func (s *LowcodeService) DeleteTable(ctx context.Context, req *apiv1.DeleteTableRequest) (*apiv1.DeleteTableResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	data, err := s.tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}

	var schemaName, tableName string
	if err := meta.QueryRow(ctx, `SELECT schema_name, table_name FROM lc_tables WHERE name = $1 AND tenant_id = $2`, req.Id, tid).
		Scan(&schemaName, &tableName); err != nil {
		if err == pgx.ErrNoRows {
			return &apiv1.DeleteTableResponse{}, nil
		}
		return nil, err
	}

	dropSQL := fmt.Sprintf(`DROP TABLE IF EXISTS %s.%s`,
		pgx.Identifier{schemaName}.Sanitize(), pgx.Identifier{tableName}.Sanitize())
	if _, err := data.Exec(ctx, dropSQL); err != nil {
		return nil, err
	}

	if _, err := meta.Exec(ctx, `DELETE FROM lc_tables WHERE name = $1 AND tenant_id = $2`, req.Id, tid); err != nil {
		return nil, err
	}
	s.invalidateTableMetaCache(ctx, req.Id)
	return &apiv1.DeleteTableResponse{}, nil
}

func (s *LowcodeService) ListTables(ctx context.Context, _ *apiv1.ListTablesRequest) (*apiv1.ListTablesResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	const q = `SELECT name, schema_name, table_name, created_at, updated_at FROM lc_tables WHERE tenant_id = $1 ORDER BY created_at`
	rows, err := meta.Query(ctx, q, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res apiv1.ListTablesResponse
	for rows.Next() {
		var t apiv1.Table
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&t.Name, &t.SchemaName, &t.TableName, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		// 对外：Table.Id 使用逻辑 name。
		t.Id = t.Name
		t.CreatedAt = createdAt
		t.UpdatedAt = updatedAt
		res.Tables = append(res.Tables, &t)
	}
	return &res, rows.Err()
}

// RenameTable 重命名逻辑表名，并同步物理表及元数据外键引用。
func (s *LowcodeService) RenameTable(ctx context.Context, req *apiv1.RenameTableRequest) (*apiv1.RenameTableResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	data, err := s.tenants.DataPool(ctx)
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
	if !logicalTableNameRe.MatchString(newName) {
		return nil, fmt.Errorf("new_name must match %s", logicalTableNameRe.String())
	}

	var exists int
	if err := meta.QueryRow(ctx, `SELECT 1 FROM lc_tables WHERE name = $1 AND tenant_id = $2`, newName, tid).Scan(&exists); err == nil {
		return nil, fmt.Errorf("table name %q already exists", newName)
	} else if err != pgx.ErrNoRows {
		return nil, err
	}

	var schemaName, oldPhys string
	if err := meta.QueryRow(ctx, `SELECT schema_name, table_name FROM lc_tables WHERE name = $1 AND tenant_id = $2`, oldName, tid).
		Scan(&schemaName, &oldPhys); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("table not found")
		}
		return nil, err
	}

	newPhys := newName

	renameSQL := fmt.Sprintf(`ALTER TABLE %s.%s RENAME TO %s`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{oldPhys}.Sanitize(),
		pgx.Identifier{newPhys}.Sanitize(),
	)
	if _, err := data.Exec(ctx, renameSQL); err != nil {
		return nil, err
	}
	metaOK := false
	defer func() {
		if !metaOK {
			rev := fmt.Sprintf(`ALTER TABLE %s.%s RENAME TO %s`,
				pgx.Identifier{schemaName}.Sanitize(),
				pgx.Identifier{newPhys}.Sanitize(),
				pgx.Identifier{oldPhys}.Sanitize(),
			)
			_, _ = data.Exec(ctx, rev)
		}
	}()

	tx, err := meta.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		ALTER TABLE lc_columns DROP CONSTRAINT IF EXISTS lc_columns_tenant_table_fk;
		ALTER TABLE lc_relations DROP CONSTRAINT IF EXISTS lc_relations_tenant_id_source_table_id_fkey;
		ALTER TABLE lc_relations DROP CONSTRAINT IF EXISTS lc_relations_tenant_id_target_table_id_fkey;
		ALTER TABLE lc_data_sources DROP CONSTRAINT IF EXISTS lc_data_sources_tenant_id_table_id_fkey;
	`); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE lc_tables SET name = $1, table_name = $2, updated_at = now() WHERE name = $3 AND tenant_id = $4
	`, newName, newPhys, oldName, tid); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE lc_columns SET table_id = $1 WHERE table_id = $2 AND tenant_id = $3`, newName, oldName, tid); err != nil {
		return nil, err
	}
	// relationship 列 config 里 target_table_id 存逻辑表名
	if _, err := tx.Exec(ctx, `
		UPDATE lc_columns
		SET config = jsonb_set(config, '{target_table_id}', to_jsonb($1::text), true),
		    updated_at = now()
		WHERE config->>'target_table_id' = $2 AND tenant_id = $3
	`, newName, oldName, tid); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_relations SET source_table_id = $1, updated_at = now()
		WHERE source_table_id = $2 AND tenant_id = $3
	`, newName, oldName, tid); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_relations SET target_table_id = $1, updated_at = now()
		WHERE target_table_id = $2 AND tenant_id = $3
	`, newName, oldName, tid); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lc_data_sources SET table_id = $1, updated_at = now()
		WHERE table_id = $2 AND tenant_id = $3
	`, newName, oldName, tid); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
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
	`); err != nil {
		return nil, err
	}

	const sel = `SELECT name, schema_name, table_name, created_at, updated_at FROM lc_tables WHERE name = $1 AND tenant_id = $2`
	row := tx.QueryRow(ctx, sel, newName, tid)
	var t apiv1.Table
	var createdAt, updatedAt time.Time
	if err := row.Scan(&t.Name, &t.SchemaName, &t.TableName, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	t.Id = t.Name
	t.CreatedAt = createdAt
	t.UpdatedAt = updatedAt

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	metaOK = true
	s.invalidateTableMetaCache(ctx, oldName)
	s.invalidateTableMetaCache(ctx, newName)
	return &apiv1.RenameTableResponse{Table: &t}, nil
}

// GetTableSchema 返回指定 table 以及其所有 columns 和 indexes。
func (s *LowcodeService) GetTableSchema(ctx context.Context, req *apiv1.GetTableSchemaRequest) (*apiv1.GetTableSchemaResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	if req.TableId == "" {
		return nil, fmt.Errorf("table_id is required")
	}

	// table
	var tbl apiv1.Table
	row := meta.QueryRow(ctx, `
		SELECT name, schema_name, table_name, created_at, updated_at
		FROM lc_tables
		WHERE name = $1 AND tenant_id = $2
	`, req.TableId, tid)
	var tblCreatedAt, tblUpdatedAt time.Time
	if err := row.Scan(&tbl.Name, &tbl.SchemaName, &tbl.TableName, &tblCreatedAt, &tblUpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("table not found")
		}
		return nil, err
	}
	// 对外：Table.Id 使用逻辑 name。
	tbl.Id = tbl.Name
	tbl.CreatedAt = tblCreatedAt
	tbl.UpdatedAt = tblUpdatedAt

	// columns
	colRows, err := meta.Query(ctx, `
		SELECT id, table_id, name, type_id, pg_column, is_nullable, position, config, created_at, updated_at
		FROM lc_columns
		WHERE table_id = $1 AND tenant_id = $2
		ORDER BY position
	`, req.TableId, tid)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	var columns []*apiv1.Column
	for colRows.Next() {
		var c apiv1.Column
		var cfg map[string]any
		var createdAt, updatedAt time.Time
		if err := colRows.Scan(&c.Id, &c.TableId, &c.Name, &c.TypeId, &c.PgColumn, &c.IsNullable, &c.Position, &cfg, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt = createdAt
		c.UpdatedAt = updatedAt
		if cfg != nil {
			c.Config = cfg
		}
		columns = append(columns, &c)
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	// indexes — read from PostgreSQL catalog
	pgIdxRows, err := s.listPGIndexes(ctx, tbl.SchemaName, tbl.TableName)
	if err != nil {
		return nil, err
	}
	indexes, err := s.pgIndexesToAPI(ctx, req.TableId, tbl.SchemaName, tbl.TableName, pgIdxRows)
	if err != nil {
		return nil, err
	}

	return &apiv1.GetTableSchemaResponse{
		Table:   &tbl,
		Columns: columns,
		Indexes: indexes,
	}, nil
}
