package postgres

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/solat/lowcode-database/internal/config"
)

// TenantManager holds the meta-database pool and per-tenant data-database pools.
// Meta DB stores all lc_* metadata (with tenant_id). Data DBs store only physical lc_t_* tables.
type TenantManager struct {
	metaPool *pgxpool.Pool

	// Optional: superuser DSN (usually .../postgres) to CREATE DATABASE when provisioning tenants.
	adminPool *pgxpool.Pool
	// Optional: printf template for data DSN when API omits data_dsn, e.g. postgresql://u:p@host:5432/%s
	dataDSNTemplate string

	poolSettings         PoolSettings
	defaultTenantPoolMax int

	mu            sync.RWMutex
	dataPools     map[string]*pgxpool.Pool // pool key -> pool
	dataPoolOrder []string
}

// NewTenantManager connects to META_DATABASE_URL. Schema must be applied via cmd/migrate or docker init.
func NewTenantManager(ctx context.Context, cfg *config.Config) (*TenantManager, error) {
	if cfg.MetaDatabaseURL == "" {
		return nil, fmt.Errorf("META_DATABASE_URL is required")
	}
	settings := PoolSettingsFromConfig(cfg)
	meta, err := NewPoolFromDSN(ctx, cfg.MetaDatabaseURL, settings, 0)
	if err != nil {
		return nil, fmt.Errorf("meta database: %w", err)
	}

	m := &TenantManager{
		metaPool:             meta,
		dataDSNTemplate:      cfg.DataDSNTemplate,
		poolSettings:         settings,
		defaultTenantPoolMax: cfg.DefaultTenantPoolMax,
		dataPools:            make(map[string]*pgxpool.Pool),
	}

	if cfg.DataAdminDatabaseURL != "" {
		ap, err := NewPoolFromDSN(ctx, cfg.DataAdminDatabaseURL, settings, 0)
		if err != nil {
			meta.Close()
			return nil, fmt.Errorf("data admin pool: %w", err)
		}
		m.adminPool = ap
	}

	// Bootstrap default tenant row (Retool-style single-stack): optional env.
	if cfg.DefaultTenantDataDSN != "" {
		tid := cfg.DefaultTenantID
		if tid == "" {
			tid = "default"
		}
		_, _ = meta.Exec(ctx, `
			INSERT INTO lc_tenants (id, display_name, data_dsn)
			VALUES ($1, $2, $3)
			ON CONFLICT (id) DO NOTHING
		`, tid, "Default", cfg.DefaultTenantDataDSN)
	}

	return m, nil
}

// Close releases pools (for tests / graceful shutdown).
func (m *TenantManager) Close() {
	if m.metaPool != nil {
		m.metaPool.Close()
	}
	if m.adminPool != nil {
		m.adminPool.Close()
	}
	m.mu.Lock()
	for _, p := range m.dataPools {
		p.Close()
	}
	m.dataPools = make(map[string]*pgxpool.Pool)
	m.dataPoolOrder = nil
	m.mu.Unlock()
}

// MetaPool returns the shared metadata database pool.
func (m *TenantManager) MetaPool() *pgxpool.Pool {
	return m.metaPool
}

func (m *TenantManager) poolMaxConns(profile tenantProfile) int {
	if profile.poolMaxConns > 0 {
		return profile.poolMaxConns
	}
	return m.defaultTenantPoolMax
}

func (m *TenantManager) touchPoolKey(key string) {
	for i, k := range m.dataPoolOrder {
		if k == key {
			m.dataPoolOrder = append(append(m.dataPoolOrder[:i], m.dataPoolOrder[i+1:]...), key)
			return
		}
	}
	m.dataPoolOrder = append(m.dataPoolOrder, key)
}

func (m *TenantManager) evictPoolIfNeeded() {
	limit := m.poolSettings.MaxTenantPools
	if limit <= 0 || len(m.dataPools) < limit {
		return
	}
	if len(m.dataPoolOrder) == 0 {
		return
	}
	key := m.dataPoolOrder[0]
	m.dataPoolOrder = m.dataPoolOrder[1:]
	if p, ok := m.dataPools[key]; ok {
		p.Close()
		delete(m.dataPools, key)
	}
}

func (m *TenantManager) getOrCreatePool(ctx context.Context, key, dsn string, maxConns int) (*pgxpool.Pool, error) {
	m.mu.RLock()
	if p, ok := m.dataPools[key]; ok {
		m.mu.RUnlock()
		m.mu.Lock()
		m.touchPoolKey(key)
		m.mu.Unlock()
		return p, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.dataPools[key]; ok {
		m.touchPoolKey(key)
		return p, nil
	}
	m.evictPoolIfNeeded()
	p, err := NewPoolFromDSN(ctx, dsn, m.poolSettings, maxConns)
	if err != nil {
		return nil, err
	}
	m.dataPools[key] = p
	m.touchPoolKey(key)
	return p, nil
}
