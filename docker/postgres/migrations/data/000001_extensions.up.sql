-- Tenant data DB: extensions only (physical lc_t_* tables created at runtime)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS "postgis";
EXCEPTION
    WHEN OTHERS THEN
        RAISE NOTICE 'postgis extension not available, skipping';
END $$;
