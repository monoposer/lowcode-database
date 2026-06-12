// Package testutil provides helpers for integration tests against PostgreSQL.
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/solat/lowcode-database/internal/config"
	"github.com/solat/lowcode-database/internal/infra/postgres"
	"github.com/solat/lowcode-database/internal/migrator"
	"github.com/solat/lowcode-database/internal/service"
	"github.com/solat/lowcode-database/internal/tenant"
)

const testTenant = "test"

// SetupIntegration creates isolated meta+data pools for integration tests.
// Skips the test when TEST_META_DATABASE_URL is unset.
func SetupIntegration(t *testing.T) (*service.LowcodeService, func()) {
	t.Helper()
	metaURL := os.Getenv("TEST_META_DATABASE_URL")
	dataURL := os.Getenv("TEST_DATA_DATABASE_URL")
	if metaURL == "" {
		t.Skip("TEST_META_DATABASE_URL not set; skipping integration test")
	}
	if dataURL == "" {
		dataURL = metaURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	metaDir, err := migrator.DefaultDir("meta")
	if err != nil {
		t.Fatalf("meta migrations dir: %v", err)
	}
	if err := migrator.Apply(ctx, metaURL, metaDir); err != nil {
		t.Fatalf("apply meta migrations: %v", err)
	}
	dataDir, err := migrator.DefaultDir("data")
	if err != nil {
		t.Fatalf("data migrations dir: %v", err)
	}
	if err := migrator.Apply(ctx, dataURL, dataDir); err != nil {
		t.Fatalf("apply data migrations: %v", err)
	}

	cfg := &config.Config{
		MetaDatabaseURL:      metaURL,
		DefaultTenantDataDSN: dataURL,
		DefaultTenantID:      testTenant,
	}
	tm, err := postgres.NewTenantManager(ctx, cfg)
	if err != nil {
		t.Fatalf("tenant manager: %v", err)
	}

	// Reset tenant-scoped meta rows for isolation.
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM lc_data_sources WHERE tenant_id = $1`, testTenant)
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM lc_relations WHERE tenant_id = $1`, testTenant)
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM lc_columns WHERE tenant_id = $1`, testTenant)
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM lc_tables WHERE tenant_id = $1`, testTenant)
	_, _ = tm.MetaPool().Exec(ctx, `
		INSERT INTO lc_tenants (id, display_name, data_dsn)
		VALUES ($1, 'Test', $2)
		ON CONFLICT (id) DO UPDATE SET data_dsn = EXCLUDED.data_dsn`, testTenant, dataURL)

	svc := service.NewLowcodeService(tm, 100, nil)
	cleanup := func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		_, _ = tm.MetaPool().Exec(cctx, `DELETE FROM lc_data_sources WHERE tenant_id = $1`, testTenant)
		_, _ = tm.MetaPool().Exec(cctx, `DELETE FROM lc_relations WHERE tenant_id = $1`, testTenant)
		_, _ = tm.MetaPool().Exec(cctx, `DELETE FROM lc_columns WHERE tenant_id = $1`, testTenant)
		_, _ = tm.MetaPool().Exec(cctx, `DELETE FROM lc_tables WHERE tenant_id = $1`, testTenant)
		tm.Close()
	}
	return svc, cleanup
}

// Ctx returns a tenant-scoped context for tests.
func Ctx() context.Context {
	return tenant.WithTenantID(context.Background(), testTenant)
}

// UniqueName generates a unique logical name for test tables.
func UniqueName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
