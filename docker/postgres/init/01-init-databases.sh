#!/bin/bash
# Schema names are not hardcoded. cmd/migrate creates the catalog schema from
# META_DATABASE_URL search_path (same idea as auth-cn DB_NAMESPACE).
# Row tables live in public (EnsureDataTables).
true
