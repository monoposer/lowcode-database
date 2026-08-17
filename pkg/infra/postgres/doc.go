// Package postgres manages meta and per-tenant data database connections via TenantManager.
//
//   - tenant_manager.go — NewTenantManager, meta/data pools
//   - record_store.go — shared vs dedicated {tenant_id}_record
//   - shard_pool.go — tenant bootstrap, ListActiveShards / ListActiveDataStores
//   - scope.go — tenant_id / base_id WHERE helpers
//   - tenant_pool.go — DataPool / DataReadPool, CreateTenant, profile
//   - pg.go — NewPoolFromDSN, pool settings from config
//   - sql_trace.go — debug-level pgx SQL tracer
package postgres
