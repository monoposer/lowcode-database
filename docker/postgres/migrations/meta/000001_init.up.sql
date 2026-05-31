-- Meta database schema (single migration for fresh installs).
-- name = logical id and PG object name; label = display name.
-- Data source columns: column_names (logical column names within table_id).
-- Column types are built-in (internal/columntype); indexes and choice enums live in tenant data DB.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS lc_tenants (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL DEFAULT '',
    data_dsn     TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS lc_tables (
    tenant_id   TEXT NOT NULL DEFAULT 'default',
    name        TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    schema_name TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, name),
    UNIQUE (tenant_id, schema_name, name)
);

CREATE TABLE IF NOT EXISTS lc_columns (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   TEXT NOT NULL DEFAULT 'default',
    table_id    TEXT NOT NULL,
    name        TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    type_id     TEXT NOT NULL,
    is_nullable BOOLEAN NOT NULL DEFAULT TRUE,
    position    INT NOT NULL,
    config      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, table_id, name),
    FOREIGN KEY (tenant_id, table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS lc_webhooks (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    TEXT NOT NULL DEFAULT 'default',
    name         TEXT NOT NULL,
    target_url   TEXT NOT NULL,
    table_filter TEXT NOT NULL DEFAULT '',
    events       JSONB NOT NULL DEFAULT '[]'::jsonb,
    headers      JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    secret       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS lc_relations (
    tenant_id         TEXT NOT NULL DEFAULT 'default',
    source_table_id   TEXT NOT NULL,
    name              TEXT NOT NULL,
    kind              TEXT NOT NULL DEFAULT 'MANY_TO_ONE',
    source_column_id  TEXT,
    target_table_id   TEXT NOT NULL,
    target_column_id  TEXT,
    config            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, source_table_id, name),
    FOREIGN KEY (tenant_id, source_table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, target_table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS lc_data_sources (
    tenant_id    TEXT NOT NULL DEFAULT 'default',
    table_id     TEXT NOT NULL,
    name         TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    filter       JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort         JSONB NOT NULL DEFAULT '[]'::jsonb,
    column_names TEXT[] NOT NULL DEFAULT '{}',
    config       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, table_id, name),
    FOREIGN KEY (tenant_id, table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE
);
