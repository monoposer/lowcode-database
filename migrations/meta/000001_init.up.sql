-- Meta database schema (single migration for fresh installs).
-- Hierarchy: Tenant ⊃ Base ⊃ Table. Row data on the tenant data DB; LIST key = vt_id.
-- name = logical API id; label = display name. tenant_id = X-Tenant-Id.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Tenant: isolation unit + data-plane DSN.
CREATE TABLE IF NOT EXISTS tenants (
    tenant_id      TEXT PRIMARY KEY,
    name           TEXT NOT NULL DEFAULT '',
    label          TEXT NOT NULL DEFAULT '',
    data_dsn       TEXT NOT NULL,
    data_dsn_write TEXT NOT NULL DEFAULT '',
    data_dsn_reads TEXT[] NOT NULL DEFAULT '{}',
    pool_max_conns INT NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'active',
    record_store   TEXT NOT NULL DEFAULT 'shared',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tenants_record_store_check CHECK (record_store IN ('shared', 'dedicated'))
);

-- Logical app/database within a tenant.
CREATE TABLE IF NOT EXISTS lc_bases (
    base_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    label TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS lc_bases_tenant_idx ON lc_bases (tenant_id);

CREATE TABLE IF NOT EXISTS lc_tables (
    tenant_id       TEXT NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    base_id     TEXT NOT NULL REFERENCES lc_bases(base_id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    vt_id       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, base_id, name)
);

CREATE UNIQUE INDEX IF NOT EXISTS lc_tables_vt_id_uidx
    ON lc_tables (vt_id) WHERE vt_id IS NOT NULL AND vt_id <> '';
CREATE INDEX IF NOT EXISTS lc_tables_tenant_idx ON lc_tables (tenant_id);
CREATE INDEX IF NOT EXISTS lc_tables_base_idx ON lc_tables (base_id);

CREATE TABLE IF NOT EXISTS lc_columns (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       TEXT NOT NULL,
    base_id     TEXT NOT NULL,
    table_id    TEXT NOT NULL,
    name        TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    type_id     TEXT NOT NULL,
    is_nullable BOOLEAN NOT NULL DEFAULT TRUE,
    position    INT NOT NULL,
    config      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, base_id, table_id, name),
    FOREIGN KEY (tenant_id, base_id, table_id) REFERENCES lc_tables(tenant_id, base_id, name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS lc_indexes (
    tenant_id           TEXT NOT NULL,
    base_id         TEXT NOT NULL,
    table_id        TEXT NOT NULL,
    name            TEXT NOT NULL,
    pg_index        TEXT NOT NULL,
    column_ids      JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_unique       BOOLEAN NOT NULL DEFAULT false,
    vt_id           TEXT NOT NULL DEFAULT '',
    index_expr      TEXT NOT NULL DEFAULT '',
    index_type      TEXT NOT NULL DEFAULT 'btree',
    migrate_status  TEXT NOT NULL DEFAULT 'ready',
    migrate_error   TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, base_id, table_id, name),
    UNIQUE (tenant_id, vt_id, pg_index),
    FOREIGN KEY (tenant_id, base_id, table_id) REFERENCES lc_tables(tenant_id, base_id, name) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS lc_indexes_tenant_table_idx ON lc_indexes (tenant_id, base_id, table_id);

CREATE TABLE IF NOT EXISTS lc_column_types (
    tenant_id        TEXT NOT NULL,
    base_id      TEXT NOT NULL REFERENCES lc_bases(base_id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    spec         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, base_id, name)
);

CREATE INDEX IF NOT EXISTS lc_column_types_tenant_idx ON lc_column_types (tenant_id);

CREATE TABLE IF NOT EXISTS lc_api_keys (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          TEXT NOT NULL DEFAULT 'default',
    name           TEXT NOT NULL,
    key_hash       TEXT NOT NULL,
    key_prefix     TEXT NOT NULL DEFAULT '',
    enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    rate_limit_rps INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE UNIQUE INDEX IF NOT EXISTS lc_api_keys_key_hash_idx ON lc_api_keys (key_hash);

CREATE TABLE IF NOT EXISTS lc_queries (
    tenant_id        TEXT NOT NULL,
    base_id      TEXT NOT NULL,
    table_id     TEXT NOT NULL,
    name         TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    filter       JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort         JSONB NOT NULL DEFAULT '[]'::jsonb,
    column_names TEXT[] NOT NULL DEFAULT '{}',
    config       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, base_id, table_id, name),
    FOREIGN KEY (tenant_id, base_id, table_id) REFERENCES lc_tables(tenant_id, base_id, name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS lc_schema_audit (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         TEXT NOT NULL DEFAULT 'default',
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id   TEXT NOT NULL DEFAULT '',
    table_id      TEXT NOT NULL DEFAULT '',
    detail        JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS lc_schema_audit_tenant_occurred_idx
    ON lc_schema_audit (tenant_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS lc_event_webhooks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     TEXT NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    url           TEXT NOT NULL,
    secret        TEXT NOT NULL DEFAULT '',
    event_prefix  TEXT NOT NULL DEFAULT '',
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS lc_event_webhooks_tenant_idx ON lc_event_webhooks (tenant_id);
