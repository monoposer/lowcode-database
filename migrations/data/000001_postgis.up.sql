-- Enable PostGIS on the tenant data DB (geometry / geography / point columns).
-- Requires PostGIS binaries on the server (Docker: postgis/postgis image).
CREATE EXTENSION IF NOT EXISTS postgis;
