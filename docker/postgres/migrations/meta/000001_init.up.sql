-- Meta database: tenant registry, table/column metadata, relations, data sources, webhooks.
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
    schema_name TEXT NOT NULL,
    table_name  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, name),
    UNIQUE (tenant_id, schema_name, table_name)
);

CREATE TABLE IF NOT EXISTS lc_columns (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   TEXT NOT NULL DEFAULT 'default',
    table_id    TEXT NOT NULL,
    name        TEXT NOT NULL,
    type_id     TEXT NOT NULL,
    pg_column   TEXT NOT NULL,
    is_nullable BOOLEAN NOT NULL DEFAULT TRUE,
    position    INT NOT NULL,
    config      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, table_id, name),
    UNIQUE (tenant_id, table_id, pg_column),
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

CREATE TABLE IF NOT EXISTS lc_data_sources (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   TEXT NOT NULL DEFAULT 'default',
    name        TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    table_id    TEXT NOT NULL,
    filter      JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort        JSONB NOT NULL DEFAULT '[]'::jsonb,
    column_ids  UUID[] NOT NULL DEFAULT '{}',
    config      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name),
    FOREIGN KEY (tenant_id, table_id) REFERENCES lc_tables(tenant_id, name) ON DELETE CASCADE
);
