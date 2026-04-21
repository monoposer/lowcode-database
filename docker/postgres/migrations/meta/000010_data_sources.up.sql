CREATE TABLE IF NOT EXISTS lc_data_sources (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   TEXT NOT NULL DEFAULT 'default',
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL DEFAULT 'TABLE',
    table_id    TEXT,
    view_id     UUID REFERENCES lc_views(id) ON DELETE SET NULL,
    filter      JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort        JSONB NOT NULL DEFAULT '[]'::jsonb,
    column_ids  UUID[] NOT NULL DEFAULT '{}',
    config      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);
