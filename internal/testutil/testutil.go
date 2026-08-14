// Package testutil provides helpers for integration tests against PostgreSQL.
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/monoposer/lowcode-database/internal/service"
	"github.com/monoposer/lowcode-database/migrations"
	"github.com/monoposer/lowcode-database/pkg/config"
	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
	"github.com/monoposer/lowcode-database/pkg/migrator"
	"github.com/monoposer/lowcode-database/pkg/tenant"
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
	_, _ = tm.MetaPool().Exec(ctx, `DELETE FROM lc_column_types WHERE tenant_id = $1`, tenantID)
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

// SharedTenantA is tenant id for two-tenant-on-one-DSN integration tests.
func SharedTenantA() string { return sharedTenantA }

// SharedTenantB is the second tenant id for two-tenant-on-one-DSN integration tests.
func SharedTenantB() string { return sharedTenantB }

const vrTenant = "vr_test"

// VRTenant is the tenant id for virtual_records integration tests.
func VRTenant() string { return vrTenant }

// SetupIntegrationVR uses a dedicated test tenant on the same record store.
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
	}
	tm, err := postgres.NewTenantManager(ctx, cfg)
	if err != nil {
		t.Fatalf("tenant manager: %v", err)
	}
	cleanupTenantMeta(ctx, tm, vrTenant)
	if err := tm.BootstrapVirtualRecordsSeeds(ctx, vrTenant, dataURL); err != nil {
		t.Fatalf("bootstrap vr: %v", err)
	}

	svc := service.NewLowcodeService(tm, 100)
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

// SetupIntegrationSharedDB boots two tenants that share one data DSN.
func SetupIntegrationSharedDB(t *testing.T) (*service.LowcodeService, func()) {
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
		DefaultTenantID:      sharedTenantA,
		VRDefaultShardDSN:    dataURL,
	}
	tm, err := postgres.NewTenantManager(ctx, cfg)
	if err != nil {
		t.Fatalf("tenant manager: %v", err)
	}
	for _, tid := range []string{sharedTenantA, sharedTenantB} {
		cleanupTenantMeta(ctx, tm, tid)
		if err := tm.BootstrapVirtualRecordsSeeds(ctx, tid, dataURL); err != nil {
			t.Fatalf("bootstrap tenant %s: %v", tid, err)
		}
	}

	svc := service.NewLowcodeService(tm, 100)
	cleanup := func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		cleanupTenantMeta(cctx, tm, sharedTenantA)
		cleanupTenantMeta(cctx, tm, sharedTenantB)
		tm.Close()
	}
	return svc, cleanup
}

// CtxTenant returns a tenant-scoped context for multi-tenant tests.
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
