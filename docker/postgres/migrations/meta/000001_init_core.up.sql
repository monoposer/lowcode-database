CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS lc_types (
    id         TEXT PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    pg_type    TEXT NOT NULL,
    config     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS lc_tables (
    name        TEXT PRIMARY KEY,
    schema_name TEXT NOT NULL,
    table_name  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (schema_name, table_name)
);

CREATE TABLE IF NOT EXISTS lc_columns (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    table_id    TEXT NOT NULL REFERENCES lc_tables(name) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    type_id     TEXT NOT NULL REFERENCES lc_types(id),
    pg_column   TEXT NOT NULL,
    is_nullable BOOLEAN NOT NULL DEFAULT TRUE,
    position    INT NOT NULL,
    config      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (table_id, name),
    UNIQUE (table_id, pg_column)
);

CREATE TABLE IF NOT EXISTS lc_indexes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    table_id   TEXT NOT NULL REFERENCES lc_tables(name) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    pg_index   TEXT NOT NULL,
    column_ids UUID[] NOT NULL,
    is_unique  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (table_id, name),
    UNIQUE (table_id, pg_index)
);

INSERT INTO lc_types (id, name, pg_type, config)
VALUES
  ('text', 'text', 'text', '{}'::jsonb),
  ('number', 'number', 'numeric', '{}'::jsonb),
  ('bool', 'bool', 'boolean', '{}'::jsonb),
  ('timestamp', 'timestamp', 'timestamptz', '{}'::jsonb),
  ('json', 'json', 'jsonb', '{}'::jsonb)
ON CONFLICT (id) DO NOTHING;
