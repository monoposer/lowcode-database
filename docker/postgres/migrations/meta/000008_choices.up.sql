CREATE TABLE IF NOT EXISTS lc_choices (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   TEXT NOT NULL DEFAULT 'default',
    name        TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    source      JSONB NOT NULL DEFAULT '{"type":"STATIC"}'::jsonb,
    values      JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);
