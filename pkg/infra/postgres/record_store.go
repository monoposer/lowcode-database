package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/pkg/tenant"
)

// Record store modes on tenants.record_store.
// Physical DB isolation is orthogonal: set tenants.data_dsn to a dedicated database.
const (
	RecordStoreShared    = "shared"    // public.record / link_ref / calc_queue
	RecordStoreDedicated = "dedicated" // {tenant_id}_record / _link_ref / _calc_queue
)

var nonIdent = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

type dataTablesCtxKey struct{}

// DataTables is the physical relation set for one tenant on its data DSN.
type DataTables struct {
	Mode      string
	TenantID  string
	Record    string
	LinkRef   string
	CalcQueue string
	CalcDLQ   string
}

func SharedDataTables() DataTables {
	return DataTables{
		Mode:      RecordStoreShared,
		Record:    RecordTable,
		LinkRef:   LinkRefTable,
		CalcQueue: CalcQueueTable,
		CalcDLQ:   "calc_dead_letter",
	}
}

// ParseRecordStore accepts API/DB values. Empty defaults to shared.
func ParseRecordStore(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", RecordStoreShared, "shared_table":
		return RecordStoreShared, nil
	case RecordStoreDedicated, "dedicated_table", "exclusive":
		return RecordStoreDedicated, nil
	default:
		return "", fmt.Errorf("recordStore must be %q or %q", RecordStoreShared, RecordStoreDedicated)
	}
}

// ResolveDataTables maps tenant id + store mode to physical table names.
func ResolveDataTables(tenantID, mode string) DataTables {
	mode, err := ParseRecordStore(mode)
	if err != nil {
		mode = RecordStoreShared
	}
	tenantID = strings.TrimSpace(tenantID)
	if mode != RecordStoreDedicated || tenantID == "" {
		t := SharedDataTables()
		t.TenantID = tenantID
		return t
	}
	p := dedicatedPrefix(tenantID)
	return DataTables{
		Mode:      RecordStoreDedicated,
		TenantID:  tenantID,
		Record:    p + "_record",
		LinkRef:   p + "_link_ref",
		CalcQueue: p + "_calc_queue",
		CalcDLQ:   p + "_calc_dead_letter",
	}
}

func dedicatedPrefix(tenantID string) string {
	s := nonIdent.ReplaceAllString(strings.ToLower(strings.TrimSpace(tenantID)), "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "t"
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "t_" + s
	}
	// PG ident max 63; longest suffix is _calc_dead_letter (17).
	const maxPrefix = 63 - len("_calc_dead_letter")
	if len(s) > maxPrefix {
		s = s[:maxPrefix]
		s = strings.Trim(s, "_")
	}
	return s
}

func (t DataTables) Shared() bool {
	return t.Mode != RecordStoreDedicated
}

func (t DataTables) QRecord() string    { return pgx.Identifier{t.Record}.Sanitize() }
func (t DataTables) QLinkRef() string   { return pgx.Identifier{t.LinkRef}.Sanitize() }
func (t DataTables) QCalcQueue() string { return pgx.Identifier{t.CalcQueue}.Sanitize() }
func (t DataTables) QCalcDLQ() string   { return pgx.Identifier{t.CalcDLQ}.Sanitize() }

func indexRelName(name, suffix string) string {
	s := "idx_" + name + "_" + suffix
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}

func (t DataTables) indexIdent(name string) string {
	return pgx.Identifier{name}.Sanitize()
}

// WithDataTables stores resolved physical tables on the request/worker context.
func WithDataTables(ctx context.Context, tables DataTables) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, dataTablesCtxKey{}, tables)
}

// TablesFromContext returns tables attached to ctx, or the shared store.
func TablesFromContext(ctx context.Context) DataTables {
	if ctx != nil {
		if t, ok := ctx.Value(dataTablesCtxKey{}).(DataTables); ok && t.Record != "" {
			return t
		}
	}
	return SharedDataTables()
}

// DataTables returns physical tables for the current X-Tenant-Id.
func (m *TenantManager) DataTables(ctx context.Context) (DataTables, error) {
	tenantID, err := m.ResolveTenantID(ctx)
	if err != nil {
		return DataTables{}, err
	}
	return m.DataTablesForTenant(ctx, tenantID)
}

// DataTablesForTenant returns physical tables for an explicit tenant id.
func (m *TenantManager) DataTablesForTenant(ctx context.Context, tenantID string) (DataTables, error) {
	p, err := m.loadProfile(ctx, tenantID)
	if err != nil {
		return DataTables{}, err
	}
	t := ResolveDataTables(tenantID, p.recordStore)
	return t, nil
}

// AttachDataTables loads the tenant store and puts it on ctx (for calc SQL helpers).
func (m *TenantManager) AttachDataTables(ctx context.Context) (context.Context, DataTables, error) {
	tables, err := m.DataTables(ctx)
	if err != nil {
		return ctx, tables, err
	}
	return WithDataTables(ctx, tables), tables, nil
}

// AttachDataTablesForTenant is AttachDataTables for a worker/ops tenant id.
func (m *TenantManager) AttachDataTablesForTenant(ctx context.Context, tenantID string) (context.Context, DataTables, error) {
	tables, err := m.DataTablesForTenant(ctx, tenantID)
	if err != nil {
		return ctx, tables, err
	}
	if tenant.ResolveTenantID(ctx) == "" {
		ctx = tenant.WithTenantID(ctx, tenantID)
	}
	return WithDataTables(ctx, tables), tables, nil
}
