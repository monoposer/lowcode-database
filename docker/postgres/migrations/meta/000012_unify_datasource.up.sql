-- View 即 DataSource：将 lc_views 迁入 lc_data_sources 后删除 view 表与冗余列。

ALTER TABLE lc_data_sources ADD COLUMN IF NOT EXISTS label TEXT NOT NULL DEFAULT '';

INSERT INTO lc_data_sources (tenant_id, name, label, table_id, filter, sort, column_ids, config, created_at, updated_at)
SELECT v.tenant_id, v.name, v.label, v.table_id, v.filter, v.sort, v.column_ids, v.config, v.created_at, v.updated_at
FROM lc_views v
WHERE NOT EXISTS (
    SELECT 1 FROM lc_data_sources ds
    WHERE ds.tenant_id = v.tenant_id AND ds.name = v.name
);

UPDATE lc_data_sources ds
SET label = v.label
FROM lc_views v
WHERE ds.tenant_id = v.tenant_id AND ds.name = v.name AND (ds.label IS NULL OR ds.label = '');

ALTER TABLE lc_data_sources DROP COLUMN IF EXISTS view_id;
ALTER TABLE lc_data_sources DROP COLUMN IF EXISTS kind;

-- 清理无 table_id 的行（旧 VIEW kind 且无 view 可解析时）
DELETE FROM lc_data_sources WHERE table_id IS NULL OR table_id = '';

ALTER TABLE lc_data_sources ALTER COLUMN table_id SET NOT NULL;

ALTER TABLE lc_data_sources DROP CONSTRAINT IF EXISTS lc_data_sources_tenant_id_table_id_fkey;
ALTER TABLE lc_data_sources
    ADD CONSTRAINT lc_data_sources_tenant_id_table_id_fkey
    FOREIGN KEY (tenant_id, table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE;

DROP TABLE IF EXISTS lc_views;
