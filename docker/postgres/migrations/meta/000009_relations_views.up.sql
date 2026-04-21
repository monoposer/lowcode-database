CREATE TABLE IF NOT EXISTS lc_relations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         TEXT NOT NULL DEFAULT 'default',
    name              TEXT NOT NULL,
    kind              TEXT NOT NULL DEFAULT 'MANY_TO_ONE',
    source_table_id   TEXT NOT NULL,
    source_column_id  UUID,
    target_table_id   TEXT NOT NULL,
    target_column_id  UUID,
    config            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name),
    FOREIGN KEY (tenant_id, source_table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, target_table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS lc_views (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   TEXT NOT NULL DEFAULT 'default',
    name        TEXT NOT NULL,
    table_id    TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    filter      JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort        JSONB NOT NULL DEFAULT '[]'::jsonb,
    column_ids  UUID[] NOT NULL DEFAULT '{}',
    config      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name),
    FOREIGN KEY (tenant_id, table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE
);
