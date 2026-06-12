package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/monoposer/lowcode-database/internal/tenant"
	"net/http"
	"strings"
)

const poolKeyReadSuffix = ":read"

// DataPool returns the tenant write data database pool for the current X-Tenant-Id.
func (m *TenantManager) DataPool(ctx context.Context) (*pgxpool.Pool, error) {
	tid := tenant.FromContext(ctx)
	if tid == "" {
		return nil, fmt.Errorf("X-Tenant-Id is required")
	}
	profile, err := m.loadProfile(ctx, tid)
	if err != nil {
		return nil, err
	}
	if profile.readOnly {
		return nil, fmt.Errorf("tenant %q is read-only", tid)
	}
	return m.getOrCreatePool(ctx, tid, profile.dataDSN, m.poolMaxConns(profile))
}

// DataReadPool returns a read pool: read_dsn when configured, otherwise data_dsn.
func (m *TenantManager) DataReadPool(ctx context.Context) (*pgxpool.Pool, error) {
	tid := tenant.FromContext(ctx)
	if tid == "" {
		return nil, fmt.Errorf("X-Tenant-Id is required")
	}
	profile, err := m.loadProfile(ctx, tid)
	if err != nil {
		return nil, err
	}
	dsn := profile.dataDSN
	key := tid
	if rs := strings.TrimSpace(profile.readDSN); rs != "" {
		dsn = rs
		key = tid + poolKeyReadSuffix
	}
	return m.getOrCreatePool(ctx, key, dsn, m.poolMaxConns(profile))
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
func (m *TenantManager) CreateTenant(ctx context.Context, id, displayName, dataDSN, readDSN string, readOnly bool, poolMaxConns int, createDatabase bool) error {
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
		INSERT INTO lc_tenants (id, display_name, data_dsn, read_dsn, read_only, pool_max_conns, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			data_dsn = EXCLUDED.data_dsn,
			read_dsn = EXCLUDED.read_dsn,
			read_only = EXCLUDED.read_only,
			pool_max_conns = EXCLUDED.pool_max_conns,
			updated_at = now()
	`, id, displayName, dataDSN, strings.TrimSpace(readDSN), readOnly, poolMaxConns); err != nil {
		return fmt.Errorf("insert lc_tenants: %w", err)
	}

	m.mu.Lock()
	for _, k := range []string{id, id + poolKeyReadSuffix} {
		if old, ok := m.dataPools[k]; ok {
			old.Close()
			delete(m.dataPools, k)
		}
	}
	m.mu.Unlock()

	p, err := NewPoolFromDSN(ctx, dataDSN, m.poolSettings, poolMaxConns)
	if err != nil {
		return fmt.Errorf("connect data db: %w", err)
	}
	m.mu.Lock()
	if old, ok := m.dataPools[id]; ok {
		old.Close()
	}
	m.dataPools[id] = p
	m.touchPoolKey(id)
	m.mu.Unlock()
	return nil
}

// ActiveDataPoolCount returns the number of cached tenant data/read pools (for observability).
func (m *TenantManager) ActiveDataPoolCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.dataPools)
}

type tenantProfile struct {
	dataDSN      string
	readDSN      string
	readOnly     bool
	poolMaxConns int
}

func (m *TenantManager) loadProfile(ctx context.Context, tid string) (tenantProfile, error) {
	tid = strings.TrimSpace(tid)
	if tid == "" {
		return tenantProfile{}, fmt.Errorf("tenant id is required")
	}
	var p tenantProfile
	err := m.metaPool.QueryRow(ctx, `
		SELECT data_dsn, COALESCE(read_dsn, ''), COALESCE(read_only, false), COALESCE(pool_max_conns, 0)
		FROM lc_tenants WHERE id = $1
	`, tid).Scan(&p.dataDSN, &p.readDSN, &p.readOnly, &p.poolMaxConns)
	if err != nil {
		if err == pgx.ErrNoRows {
			return tenantProfile{}, fmt.Errorf("unknown tenant %q (create tenant first)", tid)
		}
		return tenantProfile{}, fmt.Errorf("resolve tenant profile: %w", err)
	}
	return p, nil
}

func (m *TenantManager) IsReadOnly(ctx context.Context, tid string) (bool, error) {
	if strings.TrimSpace(tid) == "" {
		tid = tenantIDFromCtx(ctx)
	}
	if tid == "" {
		return false, fmt.Errorf("X-Tenant-Id is required")
	}
	p, err := m.loadProfile(ctx, tid)
	if err != nil {
		return false, err
	}
	return p.readOnly, nil
}

// EnsureWritable rejects mutating HTTP requests for read-only tenants.
func (m *TenantManager) EnsureWritable(ctx context.Context, method, path string) error {
	if m == nil || !isMutatingRequest(method, path) {
		return nil
	}
	tid := strings.TrimSpace(tenant.FromContext(ctx))
	if tid == "" {
		return nil
	}
	ro, err := m.IsReadOnly(ctx, tid)
	if err != nil {
		return err
	}
	if ro {
		return fmt.Errorf("tenant %q is read-only", tid)
	}
	return nil
}

func isMutatingRequest(method, path string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return strings.HasPrefix(path, "/v1/")
	}
}

func tenantIDFromCtx(ctx context.Context) string {
	return strings.TrimSpace(tenant.FromContext(ctx))
}
