# platform-services module

Cross-cutting: authn, cache, metrics, logging, tracing.

## pkg/platform/authn

API Key validation (`lc_api_keys`). Enabled when `API_KEY_REQUIRED=true`.

## pkg/platform/authz

**Not implemented.** See [roadmap](../roadmap.md#authorization-rbac).

## pkg/platform/cache

Redis metadata cache: query, column spec, etc.; invalidated on writes.

`CACHE_ENABLED` + `REDIS_URL`

## pkg/platform/metrics

The app no longer records rolling Query timings. SQL stats accumulate in Postgres **`pg_stat_statements`**.

Switch: `PG_STAT_STATEMENTS=true|false` (default false).

When on: `GET /v1/admin/pg-stat-statements?limit=100` lists statements on the current tenant **data DB** (by `total_exec_time` desc).

Postgres needs `shared_preload_libraries=pg_stat_statements` (already in `deploy/docker-compose.yml`) + data migration `000002_pg_stat_statements.up.sql`.

## pkg/logger

JSON stdout; SQL / slow-query logs.

## pkg/telemetry

OpenTelemetry traces and metrics. Default is the OTel no-op providers.

Set `OTEL_EXPORTER_OTLP_ENDPOINT` (or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` / `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`) to export over OTLP HTTP. Standard `OTEL_*` env vars apply (`OTEL_SERVICE_NAME`, headers, etc.).

## Startup wiring

Injected into `shared.Base` or the HTTP middleware chain from `cmd/server/main.go`.

## Config summary

See the env-var table in [system.md](../architecture/system.md).
