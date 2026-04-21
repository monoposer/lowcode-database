ALTER TABLE lc_choices ADD COLUMN IF NOT EXISTS schema_name TEXT NOT NULL DEFAULT 'public';
ALTER TABLE lc_choices ADD COLUMN IF NOT EXISTS pg_type_name TEXT;

INSERT INTO lc_types (id, name, pg_type, config)
VALUES ('enum', 'enum', '', '{"kind":"enum"}'::jsonb)
ON CONFLICT (id) DO UPDATE SET config = EXCLUDED.config, updated_at = now();
