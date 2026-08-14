package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/pkg/tenant"
)

// DataPool returns the tenant write pool (tenants.data_dsn / data_dsn_write).
func (m *TenantManager) DataPool(ctx context.Context) (*pgxpool.Pool, error) {
	return m.DataPoolForTenant(ctx)
}

// DataReadPool returns a replica pool when data_dsn_reads is set.
// Strong-consistency reads (X-Read-Consistency: strong) always use the write DSN.
func (m *TenantManager) DataReadPool(ctx context.Context) (*pgxpool.Pool, error) {
	tenantID, err := m.ResolveTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return m.ReadPoolForTenant(ctx, tenantID, tenant.StrongRead(ctx))
}

// EffectiveDataDSN returns the tenant write DSN.
func (m *TenantManager) EffectiveDataDSN(ctx context.Context) (string, error) {
	tenantID, err := m.ResolveTenantID(ctx)
	if err != nil {
		return "", err
	}
	p, err := m.loadProfile(ctx, tenantID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(p.writeDSN) == "" {
		return "", fmt.Errorf("tenant %q has no data_dsn", tenantID)
	}
	return p.writeDSN, nil
}

// CreateTenant registers a tenant with its data DSN and seeds a default base.
func (m *TenantManager) CreateTenant(ctx context.Context, id, displayName, dataDSN string, poolMaxConns int) error {
	return m.CreateTenantFull(ctx, id, displayName, dataDSN, nil, poolMaxConns, RecordStoreShared)
}

// CreateTenantFull registers a tenant with write DSN, optional read replica DSNs, and record store mode.
func (m *TenantManager) CreateTenantFull(ctx context.Context, id, displayName, dataDSN string, readDSNs []string, poolMaxConns int, recordStore string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("tenant id is required")
	}
	if displayName == "" {
		displayName = id
	}
	var n int
	_ = m.metaPool.QueryRow(ctx, `SELECT COUNT(*)::int FROM tenants WHERE tenant_id = $1`, id).Scan(&n)
	if n > 0 {
		return nil
	}
	_, err := m.insertTenantAndSeed(ctx, id, displayName, dataDSN, readDSNs, poolMaxConns, recordStore)
	return err
}

// ActiveDataPoolCount returns the number of cached data pools (for observability).
func (m *TenantManager) ActiveDataPoolCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.dataPools)
}

type tenantProfile struct {
	writeDSN     string
	readDSNs     []string
	poolMaxConns int
	recordStore  string
}

func (m *TenantManager) loadProfile(ctx context.Context, tenantID string) (tenantProfile, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return tenantProfile{}, fmt.Errorf("tenant id is required")
	}
	var p tenantProfile
	var dataDSN, writeDSN string
	err := m.metaPool.QueryRow(ctx, `
		SELECT data_dsn,
		       COALESCE(NULLIF(data_dsn_write, ''), data_dsn),
		       COALESCE(data_dsn_reads, '{}'),
		       pool_max_conns,
		       COALESCE(NULLIF(record_store, ''), 'shared')
		FROM tenants WHERE tenant_id = $1
	`, tenantID).Scan(&dataDSN, &writeDSN, &p.readDSNs, &p.poolMaxConns, &p.recordStore)
	if err != nil {
		err = m.metaPool.QueryRow(ctx, `
			SELECT data_dsn,
			       COALESCE(NULLIF(data_dsn_write, ''), data_dsn),
			       COALESCE(data_dsn_reads, '{}'),
			       pool_max_conns
			FROM tenants WHERE tenant_id = $1
		`, tenantID).Scan(&dataDSN, &writeDSN, &p.readDSNs, &p.poolMaxConns)
		if err != nil {
			return tenantProfile{}, fmt.Errorf("unknown tenant %q", tenantID)
		}
		p.recordStore = RecordStoreShared
	}
	p.writeDSN = strings.TrimSpace(writeDSN)
	if p.writeDSN == "" {
		p.writeDSN = strings.TrimSpace(dataDSN)
	}
	p.readDSNs = normalizeReadDSNs(p.writeDSN, p.readDSNs)
	p.recordStore, _ = ParseRecordStore(p.recordStore)
	return p, nil
}

func normalizeReadDSNs(write string, reads []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, d := range reads {
		d = strings.TrimSpace(d)
		if d == "" || d == write {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	return out
}

func pickReadDSN(write string, reads []string, rr *uint64, forcePrimary bool) string {
	if forcePrimary || len(reads) == 0 {
		return write
	}
	n := atomic.AddUint64(rr, 1)
	return reads[int((n-1)%uint64(len(reads)))]
}

// ReadPoolForTenant returns a replica pool, or the write pool when forcePrimary or no replicas.
func (m *TenantManager) ReadPoolForTenant(ctx context.Context, tenantID string, forcePrimary bool) (*pgxpool.Pool, error) {
	p, err := m.loadProfile(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.writeDSN) == "" {
		return nil, fmt.Errorf("tenant %q has no data_dsn", tenantID)
	}
	dsn := pickReadDSN(p.writeDSN, p.readDSNs, &m.readRR, forcePrimary)
	return m.getOrCreatePool(ctx, dsn, dsn, p.poolMaxConns)
}

// UpdateTenantDSNs updates write and/or read replica DSNs. Empty writeDSN leaves write unchanged.
func (m *TenantManager) UpdateTenantDSNs(ctx context.Context, tenantID, writeDSN string, readDSNs []string, updateReads bool) error {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	writeDSN = strings.TrimSpace(writeDSN)
	if writeDSN == "" && !updateReads {
		return fmt.Errorf("no DSN fields to update")
	}
	if writeDSN != "" && updateReads {
		_, err := m.metaPool.Exec(ctx, `
			UPDATE tenants
			SET data_dsn = $2, data_dsn_write = $2, data_dsn_reads = $3, updated_at = now()
			WHERE tenant_id = $1
		`, tenantID, writeDSN, readDSNs)
		return err
	}
	if writeDSN != "" {
		_, err := m.metaPool.Exec(ctx, `
			UPDATE tenants
			SET data_dsn = $2, data_dsn_write = $2, updated_at = now()
			WHERE tenant_id = $1
		`, tenantID, writeDSN)
		return err
	}
	_, err := m.metaPool.Exec(ctx, `
		UPDATE tenants SET data_dsn_reads = $2, updated_at = now() WHERE tenant_id = $1
	`, tenantID, readDSNs)
	return err
}

// EnsureWritable is a no-op: tenants are always read/write (no authz / replica split).
func (m *TenantManager) EnsureWritable(context.Context, string, string) error {
	return nil
}
