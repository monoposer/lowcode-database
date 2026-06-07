-- pg_stat_statements (contrib). Requires shared_preload_libraries=pg_stat_statements
-- on the Postgres server (Docker compose sets this). Used when PG_STAT_STATEMENTS=true.
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
