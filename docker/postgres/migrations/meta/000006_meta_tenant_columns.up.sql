-- lc_webhooks: per-tenant hooks
ALTER TABLE lc_webhooks ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE lc_webhooks DROP CONSTRAINT IF EXISTS lc_webhooks_name_key;
ALTER TABLE lc_webhooks ADD CONSTRAINT lc_webhooks_tenant_name_key UNIQUE (tenant_id, name);

-- Break FKs, then rebuild lc_tables PK with tenant_id
ALTER TABLE lc_columns DROP CONSTRAINT IF EXISTS lc_columns_table_id_fkey;
ALTER TABLE lc_indexes DROP CONSTRAINT IF EXISTS lc_indexes_table_id_fkey;
ALTER TABLE lc_tables DROP CONSTRAINT IF EXISTS lc_tables_schema_name_table_name_key;
ALTER TABLE lc_tables DROP CONSTRAINT IF EXISTS lc_tables_pkey;
ALTER TABLE lc_tables ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE lc_tables ADD PRIMARY KEY (tenant_id, name);
ALTER TABLE lc_tables ADD CONSTRAINT lc_tables_tenant_schema_table_key UNIQUE (tenant_id, schema_name, table_name);

-- lc_columns
ALTER TABLE lc_columns ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
UPDATE lc_columns AS c SET tenant_id = t.tenant_id FROM lc_tables AS t WHERE c.table_id = t.name;
ALTER TABLE lc_columns DROP CONSTRAINT IF EXISTS lc_columns_table_id_name_key;
ALTER TABLE lc_columns DROP CONSTRAINT IF EXISTS lc_columns_table_id_pg_column_key;
ALTER TABLE lc_columns ADD CONSTRAINT lc_columns_tenant_table_name_key UNIQUE (tenant_id, table_id, name);
ALTER TABLE lc_columns ADD CONSTRAINT lc_columns_tenant_table_pgcol_key UNIQUE (tenant_id, table_id, pg_column);
ALTER TABLE lc_columns ADD CONSTRAINT lc_columns_tenant_table_fk FOREIGN KEY (tenant_id, table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE;

-- lc_indexes (legacy mirror table; indexes live in PG catalog at runtime)
ALTER TABLE lc_indexes ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
UPDATE lc_indexes AS i SET tenant_id = t.tenant_id FROM lc_tables AS t WHERE i.table_id = t.name;
ALTER TABLE lc_indexes DROP CONSTRAINT IF EXISTS lc_indexes_table_id_name_key;
ALTER TABLE lc_indexes DROP CONSTRAINT IF EXISTS lc_indexes_table_id_pg_index_key;
ALTER TABLE lc_indexes ADD CONSTRAINT lc_indexes_tenant_table_name_key UNIQUE (tenant_id, table_id, name);
ALTER TABLE lc_indexes ADD CONSTRAINT lc_indexes_tenant_table_pgidx_key UNIQUE (tenant_id, table_id, pg_index);
ALTER TABLE lc_indexes ADD CONSTRAINT lc_indexes_tenant_table_fk FOREIGN KEY (tenant_id, table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE;
