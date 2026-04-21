-- Column types are built-in (internal/columntype); drop lc_types registry table.

ALTER TABLE lc_columns DROP CONSTRAINT IF EXISTS lc_columns_type_id_fkey;

DROP TABLE IF EXISTS lc_types;
