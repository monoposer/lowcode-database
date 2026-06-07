#!/bin/bash
# Create empty application databases only. Schema is applied later via cmd/migrate
# (make migrate / docker compose run --rm migrate). Do not apply SQL here.
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    SELECT 'CREATE DATABASE lowcode_meta'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'lowcode_meta')\gexec
    SELECT 'CREATE DATABASE lowcode_data'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'lowcode_data')\gexec
EOSQL
