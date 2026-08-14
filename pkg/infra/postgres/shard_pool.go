package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/pkg/tenant"
)

// BootstrapVirtualRecordsSeeds registers the default tenant and base.
// tenantID is the tenant id (X-Tenant-Id).
func (m *TenantManager) BootstrapVirtualRecordsSeeds(ctx context.Context, tenantID, dataDSN string) error {
	if m == nil || m.metaPool == nil {
		return nil
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	dataDSN = strings.TrimSpace(dataDSN)
	if dataDSN == "" {
		return fmt.Errorf("data DSN is required for tenant bootstrap")
	}

	if _, err := m.metaPool.Exec(ctx, `
		INSERT INTO tenants (tenant_id, name, label, data_dsn, data_dsn_write, status)
		VALUES ($1, $2, $2, $3, $3, 'active')
		ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID, "Default", dataDSN); err != nil {
		return fmt.Errorf("seed tenants: %w", err)
	}

	baseID := "base_" + tenantID
	if _, err := m.metaPool.Exec(ctx, `
		INSERT INTO lc_bases (base_id, tenant_id, name, label, status)
		VALUES ($1, $2, 'default', 'Default', 'active')
		ON CONFLICT (base_id) DO NOTHING
	`, baseID, tenantID); err != nil {
		return fmt.Errorf("seed lc_bases: %w", err)
	}

	pool, err := m.PoolForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	return EnsureVirtualRecordsParent(ctx, pool)
}

// ResolveTenantID returns X-Tenant-Id.
func (m *TenantManager) ResolveTenantID(ctx context.Context) (string, error) {
	id := tenant.ResolveTenantID(ctx)
	if id == "" {
		return "", fmt.Errorf("X-Tenant-Id is required")
	}
	var found string
	err := m.metaPool.QueryRow(ctx, `
		SELECT tenant_id FROM tenants WHERE tenant_id = $1
	`, id).Scan(&found)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("unknown tenant %q", id)
		}
		return "", err
	}
	return found, nil
}

// ResolveBaseID returns X-Base-Id or the tenant's first base.
func (m *TenantManager) ResolveBaseID(ctx context.Context, tenantID string) (string, error) {
	if base := strings.TrimSpace(tenant.BaseFromContext(ctx)); base != "" {
		var found string
		err := m.metaPool.QueryRow(ctx, `
			SELECT base_id FROM lc_bases WHERE base_id = $1 AND tenant_id = $2
		`, base, tenantID).Scan(&found)
		if err != nil {
			if err == pgx.ErrNoRows {
				return "", fmt.Errorf("unknown base %q for tenant %q", base, tenantID)
			}
			return "", err
		}
		return found, nil
	}
	var found string
	err := m.metaPool.QueryRow(ctx, `
		SELECT base_id FROM lc_bases WHERE tenant_id = $1 ORDER BY created_at ASC LIMIT 1
	`, tenantID).Scan(&found)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("tenant %q has no bases", tenantID)
		}
		return "", err
	}
	return found, nil
}

// PoolForTenant returns the write data pool for a tenant.
func (m *TenantManager) PoolForTenant(ctx context.Context, tenantID string) (*pgxpool.Pool, error) {
	p, err := m.loadProfile(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.writeDSN) == "" {
		return nil, fmt.Errorf("tenant %q has no data_dsn", tenantID)
	}
	return m.getOrCreatePool(ctx, p.writeDSN, p.writeDSN, p.poolMaxConns)
}

// ShardPoolByTag is a compat alias: tag is the tenant tenant_id.
func (m *TenantManager) ShardPoolByTag(ctx context.Context, tenantID string) (*pgxpool.Pool, error) {
	return m.PoolForTenant(ctx, tenantID)
}

// DataPoolForTenant returns the data pool for the current request tenant.
func (m *TenantManager) DataPoolForTenant(ctx context.Context) (*pgxpool.Pool, error) {
	tenantID, err := m.ResolveTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return m.PoolForTenant(ctx, tenantID)
}

func (m *TenantManager) resolveCreateDSN(ctx context.Context, tenantID, dataDSN string) (string, error) {
	dataDSN = strings.TrimSpace(dataDSN)
	if dataDSN == "" && m.dataDSNTemplate != "" {
		dataDSN = fmt.Sprintf(m.dataDSNTemplate, tenantID)
	}
	if dataDSN != "" {
		return dataDSN, nil
	}
	var existing string
	err := m.metaPool.QueryRow(ctx, `
		SELECT data_dsn FROM tenants WHERE data_dsn <> '' ORDER BY created_at ASC LIMIT 1
	`).Scan(&existing)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("data_dsn is required")
		}
		return "", err
	}
	return existing, nil
}

// insertTenantAndSeed registers a tenant and seeds a default base.
func (m *TenantManager) insertTenantAndSeed(ctx context.Context, tenantID, name, dataDSN string, readDSNs []string, poolMaxConns int, recordStore string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "tenant_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	if name == "" {
		name = tenantID
	}
	resolved, err := m.resolveCreateDSN(ctx, tenantID, dataDSN)
	if err != nil {
		return "", err
	}
	if readDSNs == nil {
		readDSNs = []string{}
	}
	store, err := ParseRecordStore(recordStore)
	if err != nil {
		return "", err
	}
	if _, err := m.metaPool.Exec(ctx, `
		INSERT INTO tenants (tenant_id, name, label, data_dsn, data_dsn_write, data_dsn_reads, pool_max_conns, status, record_store)
		VALUES ($1, $2, $2, $3, $3, $4, $5, 'active', $6)
	`, tenantID, name, resolved, readDSNs, poolMaxConns, store); err != nil {
		return "", fmt.Errorf("insert tenant: %w", err)
	}

	baseID := "base_" + tenantID
	if _, err := m.metaPool.Exec(ctx, `
		INSERT INTO lc_bases (base_id, tenant_id, name, label, status)
		VALUES ($1, $2, 'default', 'Default', 'active')
		ON CONFLICT (base_id) DO NOTHING
	`, baseID, tenantID); err != nil {
		return "", fmt.Errorf("seed default base: %w", err)
	}

	pool, err := m.PoolForTenant(ctx, tenantID)
	if err != nil {
		return "", err
	}
	tables := ResolveDataTables(tenantID, store)
	if err := EnsureDataTables(ctx, pool, tables); err != nil {
		return "", err
	}
	return tenantID, nil
}

// CreateBase creates a logical base under a tenant.
func (m *TenantManager) CreateBase(ctx context.Context, tenantID, baseID, name, label string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return "", fmt.Errorf("tenant_id is required")
	}
	baseID = strings.TrimSpace(baseID)
	if baseID == "" {
		baseID = "base_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = baseID
	}
	if label == "" {
		label = name
	}
	if _, err := m.metaPool.Exec(ctx, `
		INSERT INTO lc_bases (base_id, tenant_id, name, label, status)
		VALUES ($1, $2, $3, $4, 'active')
	`, baseID, tenantID, name, label); err != nil {
		return "", fmt.Errorf("insert base: %w", err)
	}
	return baseID, nil
}

// ListBases returns bases for a tenant.
func (m *TenantManager) ListBases(ctx context.Context, tenantID string) ([]BaseInfo, error) {
	rows, err := m.metaPool.Query(ctx, `
		SELECT base_id, tenant_id, name, label, status
		FROM lc_bases WHERE tenant_id = $1 ORDER BY created_at
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BaseInfo
	for rows.Next() {
		var b BaseInfo
		if err := rows.Scan(&b.BaseID, &b.TenantID, &b.Name, &b.Label, &b.Status); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BaseInfo is a meta base row.
type BaseInfo struct {
	BaseID   string
	TenantID string
	Name     string
	Label    string
	Status   string
}

// ListTenants returns all tenants (tenantIDFilter empty) or a single tenant.
func (m *TenantManager) ListTenants(ctx context.Context, tenantIDFilter string) ([]TenantInfo, error) {
	var rows pgx.Rows
	var err error
	if strings.TrimSpace(tenantIDFilter) == "" {
		rows, err = m.metaPool.Query(ctx, `
			SELECT tenant_id, name, status, COALESCE(array_length(data_dsn_reads, 1), 0),
			       COALESCE(NULLIF(record_store, ''), 'shared')
			FROM tenants ORDER BY created_at
		`)
	} else {
		rows, err = m.metaPool.Query(ctx, `
			SELECT tenant_id, name, status, COALESCE(array_length(data_dsn_reads, 1), 0),
			       COALESCE(NULLIF(record_store, ''), 'shared')
			FROM tenants WHERE tenant_id = $1 ORDER BY created_at
		`, tenantIDFilter)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TenantInfo
	for rows.Next() {
		var t TenantInfo
		if err := rows.Scan(&t.TenantID, &t.Name, &t.Status, &t.ReadReplicaCount, &t.RecordStore); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TenantInfo is a meta tenants row.
type TenantInfo struct {
	TenantID         string
	Name             string
	Status           string
	ReadReplicaCount int
	RecordStore      string
}

// ShardDSN is a unique tenant data DSN used by calc workers.
type ShardDSN struct {
	Tag string // representative tenant_id
	DSN string
}

// ListActiveShards returns unique tenant data_dsn values for worker loops.
func (m *TenantManager) ListActiveShards(ctx context.Context) ([]ShardDSN, error) {
	if m == nil || m.metaPool == nil {
		return nil, fmt.Errorf("meta pool is required")
	}
	rows, err := m.metaPool.Query(ctx, `
		SELECT MIN(tenant_id), data_dsn FROM tenants
		WHERE status = 'active' AND data_dsn <> ''
		GROUP BY data_dsn
		ORDER BY MIN(tenant_id)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShardDSN
	for rows.Next() {
		var s ShardDSN
		if err := rows.Scan(&s.Tag, &s.DSN); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DataStoreTarget is one calc/index worker drain target (shared table per DSN, or a dedicated tenant table).
type DataStoreTarget struct {
	TenantID string
	DSN      string
	Tables   DataTables
}

// ListActiveDataStores returns drain targets: one shared store per DSN plus each dedicated tenant.
func (m *TenantManager) ListActiveDataStores(ctx context.Context) ([]DataStoreTarget, error) {
	if m == nil || m.metaPool == nil {
		return nil, fmt.Errorf("meta pool is required")
	}
	rows, err := m.metaPool.Query(ctx, `
		SELECT tenant_id, data_dsn, COALESCE(NULLIF(record_store, ''), 'shared')
		FROM tenants
		WHERE status = 'active' AND data_dsn <> ''
		ORDER BY tenant_id`)
	if err != nil {
		shards, err2 := m.ListActiveShards(ctx)
		if err2 != nil {
			return nil, err
		}
		out := make([]DataStoreTarget, 0, len(shards))
		for _, s := range shards {
			out = append(out, DataStoreTarget{TenantID: s.Tag, DSN: s.DSN, Tables: SharedDataTables()})
		}
		return out, nil
	}
	defer rows.Close()
	type row struct {
		id, dsn, store string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.dsn, &r.store); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	seenSharedDSN := map[string]struct{}{}
	var out []DataStoreTarget
	for _, r := range list {
		store, _ := ParseRecordStore(r.store)
		tables := ResolveDataTables(r.id, store)
		if tables.Shared() {
			if _, ok := seenSharedDSN[r.dsn]; ok {
				continue
			}
			seenSharedDSN[r.dsn] = struct{}{}
		}
		out = append(out, DataStoreTarget{TenantID: r.id, DSN: r.dsn, Tables: tables})
	}
	return out, nil
}

// TableVTID returns the global vt_id for a logical table in the given tenant/base.
func (m *TenantManager) TableVTID(ctx context.Context, tenantID, baseID, tableName string) (string, error) {
	var vtID string
	var err error
	if strings.TrimSpace(baseID) != "" {
		err = m.metaPool.QueryRow(ctx, `
			SELECT vt_id::text FROM lc_tables
			WHERE tenant_id = $1 AND base_id = $2 AND name = $3
		`, tenantID, baseID, tableName).Scan(&vtID)
	} else {
		err = m.metaPool.QueryRow(ctx, `
			SELECT vt_id::text FROM lc_tables
			WHERE tenant_id = $1 AND name = $2
			ORDER BY created_at ASC LIMIT 1
		`, tenantID, tableName).Scan(&vtID)
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("table %q not found", tableName)
		}
		return "", err
	}
	if vtID == "" {
		return "", fmt.Errorf("table %q has no vt_id (not provisioned for virtual_records)", tableName)
	}
	return vtID, nil
}
