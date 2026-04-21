package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/solat/lowcode-database/internal/config"
	"github.com/solat/lowcode-database/internal/tenant"
)

// TenantManager holds the meta-database pool and per-tenant data-database pools.
// Meta DB stores all lc_* metadata (with tenant_id). Data DBs store only physical lc_t_* tables.
type TenantManager struct {
	metaPool *pgxpool.Pool

	// Optional: superuser DSN (usually .../postgres) to CREATE DATABASE when provisioning tenants.
	adminPool *pgxpool.Pool
	// Optional: printf template for data DSN when API omits data_dsn, e.g. postgresql://u:p@host:5432/%s
	dataDSNTemplate string

	mu        sync.RWMutex
	dataPools map[string]*pgxpool.Pool // tenant id -> pool
}

// NewTenantManager connects to META_DATABASE_URL. Schema must be applied via cmd/migrate or docker init.
func NewTenantManager(ctx context.Context, cfg *config.Config) (*TenantManager, error) {
	if cfg.MetaDatabaseURL == "" {
		return nil, fmt.Errorf("META_DATABASE_URL is required")
	}
	meta, err := NewPoolFromDSN(ctx, cfg.MetaDatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("meta database: %w", err)
	}

	m := &TenantManager{
		metaPool:          meta,
		dataDSNTemplate:   cfg.DataDSNTemplate,
		dataPools:         make(map[string]*pgxpool.Pool),
	}

	if cfg.DataAdminDatabaseURL != "" {
		ap, err := NewPoolFromDSN(ctx, cfg.DataAdminDatabaseURL)
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
	m.mu.Unlock()
}

// MetaPool returns the shared metadata database pool.
func (m *TenantManager) MetaPool() *pgxpool.Pool {
	return m.metaPool
}

// DataPool returns the tenant data database pool for the current X-Tenant-Id.
func (m *TenantManager) DataPool(ctx context.Context) (*pgxpool.Pool, error) {
	tid := tenant.FromContext(ctx)
	if tid == "" {
		return nil, fmt.Errorf("X-Tenant-Id is required")
	}
	m.mu.RLock()
	if p, ok := m.dataPools[tid]; ok {
		m.mu.RUnlock()
		return p, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.dataPools[tid]; ok {
		return p, nil
	}

	var dsn string
	if err := m.metaPool.QueryRow(ctx, `SELECT data_dsn FROM lc_tenants WHERE id = $1`, tid).Scan(&dsn); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("unknown tenant %q (create tenant first)", tid)
		}
		return nil, fmt.Errorf("resolve tenant data dsn: %w", err)
	}
	p, err := NewPoolFromDSN(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("data pool for tenant %q: %w", tid, err)
	}
	m.dataPools[tid] = p
	return p, nil
}

// EffectiveDataDSN returns the raw data DSN for the current tenant (from lc_tenants).
func (m *TenantManager) EffectiveDataDSN(ctx context.Context) (string, error) {
	tid := tenant.FromContext(ctx)
	if tid == "" {
		return "", fmt.Errorf("X-Tenant-Id is required")
	}
	var dsn string
	if err := m.metaPool.QueryRow(ctx, `SELECT data_dsn FROM lc_tenants WHERE id = $1`, tid).Scan(&dsn); err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("unknown tenant %q", tid)
		}
		return "", err
	}
	return dsn, nil
}

// CreateTenant registers a tenant in meta and optionally creates a Postgres database.
// dataDSN: full connection string to the tenant data database (required unless template fills it).
func (m *TenantManager) CreateTenant(ctx context.Context, id, displayName, dataDSN string, createDatabase bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("tenant id is required")
	}
	if displayName == "" {
		displayName = id
	}
	if dataDSN == "" && m.dataDSNTemplate != "" {
		dataDSN = fmt.Sprintf(m.dataDSNTemplate, id)
	}
	if dataDSN == "" {
		return fmt.Errorf("data_dsn is required (or configure DATA_DSN_TEMPLATE)")
	}

	if createDatabase {
		if m.adminPool == nil {
			return fmt.Errorf("create_database requires DATA_ADMIN_DATABASE_URL")
		}
		createStmt := fmt.Sprintf(`CREATE DATABASE %s`, (pgx.Identifier{id}).Sanitize())
		if _, err := m.adminPool.Exec(ctx, createStmt); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "42P04" {
				// exists
			} else {
				return fmt.Errorf("create database %q: %w", id, err)
			}
		}
	}

	if _, err := m.metaPool.Exec(ctx, `
		INSERT INTO lc_tenants (id, display_name, data_dsn, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name, data_dsn = EXCLUDED.data_dsn, updated_at = now()
	`, id, displayName, dataDSN); err != nil {
		return fmt.Errorf("insert lc_tenants: %w", err)
	}

	// Warm data pool.
	p, err := NewPoolFromDSN(ctx, dataDSN)
	if err != nil {
		return fmt.Errorf("connect data db: %w", err)
	}
	m.mu.Lock()
	if old, ok := m.dataPools[id]; ok {
		old.Close()
	}
	m.dataPools[id] = p
	m.mu.Unlock()
	return nil
}
