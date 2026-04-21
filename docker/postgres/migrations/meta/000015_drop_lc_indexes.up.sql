-- Indexes live in tenant data DB (pg_index/pg_class); drop legacy meta mirror.

DROP TABLE IF EXISTS lc_indexes;
