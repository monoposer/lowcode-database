INSERT INTO lc_types (id, name, pg_type, config)
VALUES
  ('int8', 'int8', 'bigint', '{}'::jsonb),
  ('double', 'double', 'double precision', '{}'::jsonb),
  ('precision', 'precision', 'numeric', '{"precision":20,"scale":6}'::jsonb),
  ('text', 'text', 'text', '{}'::jsonb),
  ('timestamptz', 'timestamptz', 'timestamptz', '{}'::jsonb),
  ('bool', 'bool', 'boolean', '{}'::jsonb),
  ('jsonb', 'jsonb', 'jsonb', '{}'::jsonb),
  ('int8_array', 'int8_array', 'bigint[]', '{"array":true}'::jsonb),
  ('double_array', 'double_array', 'double precision[]', '{"array":true}'::jsonb),
  ('text_array', 'text_array', 'text[]', '{"array":true}'::jsonb),
  ('bool_array', 'bool_array', 'boolean[]', '{"array":true}'::jsonb),
  ('jsonb_array', 'jsonb_array', 'jsonb[]', '{"array":true}'::jsonb),
  ('timestamptz_array', 'timestamptz_array', 'timestamptz[]', '{"array":true}'::jsonb),
  ('uuid_array', 'uuid_array', 'uuid[]', '{"array":true}'::jsonb),
  ('geometry', 'geometry', 'geometry', '{"postgis":true}'::jsonb),
  ('geography', 'geography', 'geography', '{"postgis":true}'::jsonb),
  ('point', 'point', 'geometry(Point,4326)', '{"postgis":true}'::jsonb),
  ('rollup', 'rollup', 'jsonb', '{"kind":"rollup"}'::jsonb),
  ('relation_fk', 'relation_fk', 'bigint', '{"kind":"relation_fk"}'::jsonb)
ON CONFLICT (id) DO UPDATE SET
  pg_type = EXCLUDED.pg_type,
  config = EXCLUDED.config,
  updated_at = now();
