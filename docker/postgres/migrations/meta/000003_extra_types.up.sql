INSERT INTO lc_types (id, name, pg_type, config)
VALUES
  ('uuid', 'uuid', 'uuid', '{}'::jsonb),
  ('integer', 'integer', 'bigint', '{}'::jsonb),
  ('date', 'date', 'date', '{}'::jsonb),
  ('bytea', 'bytea', 'bytea', '{}'::jsonb)
ON CONFLICT (id) DO NOTHING;
