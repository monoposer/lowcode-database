// Package postgres manages meta and per-tenant data database connections via TenantManager.
//
//   - tenant_manager.go — NewTenantManager, meta pool, pool cache eviction
//   - tenant_pool.go — DataPool, read pool, CreateTenant, profile, EnsureWritable
//   - pg.go — NewPoolFromDSN, pool settings from config
package postgres
