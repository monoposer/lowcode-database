CREATE TABLE IF NOT EXISTS lc_tenants (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL DEFAULT '',
    data_dsn     TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
