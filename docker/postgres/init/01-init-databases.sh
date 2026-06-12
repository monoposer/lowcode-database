#!/bin/bash
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    SELECT 'CREATE DATABASE lowcode_meta'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'lowcode_meta')\gexec
    SELECT 'CREATE DATABASE lowcode_data'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'lowcode_data')\gexec
EOSQL

ensure_migrations_table() {
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" -d "$1" <<-EOSQL
        CREATE TABLE IF NOT EXISTS lc_schema_migrations (
            version    INT PRIMARY KEY,
            name       TEXT NOT NULL,
            applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
        );
EOSQL
}

ensure_migrations_table lowcode_meta
ensure_migrations_table lowcode_data

record_migration() {
    local db=$1 version=$2 name=$3
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" -d "$db" <<-EOSQL
        INSERT INTO lc_schema_migrations (version, name)
        VALUES ($version, '$name')
        ON CONFLICT (version) DO NOTHING;
EOSQL
}

apply_dir() {
    local db=$1 dir=$2
    for f in $(ls "$dir"/[0-9]*.up.sql 2>/dev/null | sort); do
        base=$(basename "$f")
        version=$(echo "$base" | cut -d_ -f1 | sed 's/^0*//')
        name=$(echo "$base" | sed 's/^[0-9]*_//' | sed 's/.up.sql$//')
        echo "==> $db: applying $base"
        psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" -d "$db" -f "$f"
        record_migration "$db" "$version" "$name"
    done
}

if [ -d /migrations/meta ]; then
    apply_dir lowcode_meta /migrations/meta
fi
if [ -d /migrations/data ]; then
    apply_dir lowcode_data /migrations/data
fi

# PostGIS: extension binaries come from the Docker image / OS packages; must be enabled per database.
echo "==> lowcode_data: CREATE EXTENSION postgis (optional geo columns)"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" -d lowcode_data <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS postgis;
EOSQL

# Bootstrap default tenant (matches DEFAULT_TENANT_ID=default)
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" -d lowcode_meta <<-EOSQL
    INSERT INTO lc_tenants (id, display_name, data_dsn)
    VALUES (
        'default',
        'Default',
        'postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5432/lowcode_data'
    )
    ON CONFLICT (id) DO NOTHING;
EOSQL

echo "lowcode-database postgres init complete"
