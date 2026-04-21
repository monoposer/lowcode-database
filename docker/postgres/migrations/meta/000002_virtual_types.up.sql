INSERT INTO lc_types (id, name, pg_type, config)
VALUES
  ('formula', 'formula', 'jsonb', '{"kind":"formula"}'::jsonb),
  ('relationship', 'relationship', 'jsonb', '{"kind":"relationship"}'::jsonb),
  ('lookup', 'lookup', 'jsonb', '{"kind":"lookup"}'::jsonb)
ON CONFLICT (id) DO NOTHING;
