CREATE TABLE IF NOT EXISTS lc_webhooks (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    target_url   TEXT NOT NULL,
    table_filter TEXT NOT NULL DEFAULT '',
    events       JSONB NOT NULL DEFAULT '[]'::jsonb,
    headers      JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    secret       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name)
);
