package postgres

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/singleflight"

	"github.com/monoposer/lowcode-database/pkg/config"
)

// TenantManager holds the meta-database pool and per-tenant data-database pools.
// Meta DB stores tenants (data_dsn) and lc_* metadata. Data DBs store record / link_ref / calc_queue.
type TenantManager struct {
	metaPool *pgxpool.Pool

	// Optional: superuser DSN (usually .../postgres) to CREATE DATABASE when provisioning tenants.
	adminPool *pgxpool.Pool
	// Optional: printf template for data DSN when API omits data_dsn, e.g. postgresql://u:p@host:5432/%s
	dataDSNTemplate string

	poolSettings         PoolSettings
	defaultTenantPoolMax int

	mu            sync.RWMutex
	dataPools     map[string]*poolEntry // data_dsn -> pool (shared across tenants)
	dataPoolOrder []string
	poolSF        singleflight.Group
	createWaiters int32
	readRR        uint64
}

// NewTenantManager connects to META_DATABASE_URL. Schema must be applied via cmd/migrate.
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
		dataPools:            make(map[string]*poolEntry),
	}

	if cfg.DataAdminDatabaseURL != "" {
		ap, err := NewPoolFromDSN(ctx, cfg.DataAdminDatabaseURL, settings, 0)
		if err != nil {
			meta.Close()
			return nil, fmt.Errorf("data admin pool: %w", err)
		}
		m.adminPool = ap
	}

	// Bootstrap default tenant + virtual_records shard (optional env).
	if cfg.DefaultTenantDataDSN != "" {
		tenantID := cfg.DefaultTenantID
		if tenantID == "" {
			tenantID = "default"
		}
		shardDSN := cfg.VRDefaultShardDSN
		if shardDSN == "" {
			shardDSN = cfg.DefaultTenantDataDSN
		}
		_ = m.BootstrapVirtualRecordsSeeds(ctx, tenantID, shardDSN)
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
	for _, e := range m.dataPools {
		if e != nil && e.pool != nil {
			e.pool.Close()
		}
	}
	m.dataPools = make(map[string]*poolEntry)
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
