// Package testutil provides helpers for integration tests against PostgreSQL.
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/monoposer/lowcode-database/internal/config"
	"github.com/monoposer/lowcode-database/internal/infra/postgres"
	"github.com/monoposer/lowcode-database/internal/migrator"
	"github.com/monoposer/lowcode-database/internal/service"
	"github.com/monoposer/lowcode-database/internal/tenant"
	"github.com/monoposer/lowcode-database/migrations"
)

const testTenant = "test"

func testBaseID(tenantID string) string { return "base_" + tenantID }

func applyEmbeddedMigrations(t *testing.T, ctx context.Context, metaURL, dataURL string) {
	t.Helper()
	for _, spec := range []struct {
		target string
		url    string
	}{
		{"meta", metaURL},
		{"data", dataURL},
	} {
		fsys, err := migrations.FS(spec.target)
		if err != nil {
			t.Fatalf("%s migrations: %v", spec.target, err)
		}
		if err := migrator.Apply(ctx, spec.url, fsys); err != nil {
			t.Fatalf("apply %s migrations: %v", spec.target, err)
		}
	}
}

func cleanupTenantMeta(ctx context.Context, tm *postgres.TenantManager, tenantID string) {
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM lc_indexes WHERE tenant_id = $1`, tenantID)
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM lc_queries WHERE tenant_id = $1`, tenantID)
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM lc_columns WHERE tenant_id = $1`, tenantID)
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM lc_tables WHERE tenant_id = $1`, tenantID)
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM lc_bases WHERE tenant_id = $1`, tenantID)
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM tenants WHERE tenant_id = $1`, tenantID)
}

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

	applyEmbeddedMigrations(t, ctx, metaURL, dataURL)

	cfg := &config.Config{
		MetaDatabaseURL:      metaURL,
		DefaultTenantDataDSN: dataURL,
		DefaultTenantID:      testTenant,
		VRDefaultShardDSN:    dataURL,
		TenantIsolationMode:  config.TenantIsolationRLSTable,
	}
	tm, err := postgres.NewTenantManager(ctx, cfg)
	if err != nil {
		t.Fatalf("tenant manager: %v", err)
	}

	cleanupTenantMeta(ctx, tm, testTenant)
	if err := tm.BootstrapVirtualRecordsSeeds(ctx, testTenant, dataURL); err != nil {
		t.Fatalf("bootstrap vr: %v", err)
	}

	svc := service.NewLowcodeService(tm, 100)
	cleanup := func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		cleanupTenantMeta(cctx, tm, testTenant)
		tm.Close()
	}
	return svc, cleanup
}

const sharedTenantA = "shared_a"
const sharedTenantB = "shared_b"

// SharedTenantA is tenant id for shared_db integration tests.
func SharedTenantA() string { return sharedTenantA }

// SharedTenantB is the second tenant id for shared_db integration tests.
func SharedTenantB() string { return sharedTenantB }

const vrTenant = "vr_test"

// VRTenant is the tenant id for virtual_records integration tests.
func VRTenant() string { return vrTenant }

// SetupIntegrationVR runs tests in virtual_records (rls_table) mode.
func SetupIntegrationVR(t *testing.T) (*service.LowcodeService, func()) {
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

	applyEmbeddedMigrations(t, ctx, metaURL, dataURL)

	cfg := &config.Config{
		MetaDatabaseURL:      metaURL,
		DefaultTenantDataDSN: dataURL,
		DefaultTenantID:      vrTenant,
		VRDefaultShardDSN:    dataURL,
		TenantIsolationMode:  config.TenantIsolationRLSTable,
	}
	tm, err := postgres.NewTenantManager(ctx, cfg)
	if err != nil {
		t.Fatalf("tenant manager: %v", err)
	}
	cleanupTenantMeta(ctx, tm, vrTenant)
	if err := tm.BootstrapVirtualRecordsSeeds(ctx, vrTenant, dataURL); err != nil {
		t.Fatalf("bootstrap vr: %v", err)
	}

	svc := service.NewLowcodeService(tm, 100,
		service.WithTenantIsolation(config.TenantIsolationRLSTable, ""),
	)
	cleanup := func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		cleanupTenantMeta(cctx, tm, vrTenant)
		tm.Close()
	}
	return svc, cleanup
}

// CtxVR returns a context with VR tenant + base headers.
func CtxVR() context.Context {
	ctx := tenant.WithTenantID(context.Background(), vrTenant)
	return tenant.WithBaseID(ctx, testBaseID(vrTenant))
}

// SetupIntegrationSharedDB is obsolete (shared_db mode removed); skips.
func SetupIntegrationSharedDB(t *testing.T) (*service.LowcodeService, func()) {
	t.Helper()
	t.Skip("shared_db isolation mode removed; storage is always virtual_records")
	return nil, func() {}
}

// CtxTenant returns a tenant-scoped context for shared_db tests.
func CtxTenant(tenantID string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	return tenant.WithBaseID(ctx, testBaseID(tenantID))
}

// Ctx returns a tenant + base context for tests.
func Ctx() context.Context {
	ctx := tenant.WithTenantID(context.Background(), testTenant)
	return tenant.WithBaseID(ctx, testBaseID(testTenant))
}

// UniqueName generates a unique logical name for test tables.
func UniqueName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
