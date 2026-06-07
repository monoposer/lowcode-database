package tenant

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/config"
)

func TestDataSchemaName(t *testing.T) {
	tests := []struct {
		mode   config.TenantIsolationMode
		tid    string
		prefix string
		want   string
	}{
		{config.TenantIsolationDedicatedDB, "default", "tenant_", "public"},
		{config.TenantIsolationSharedDB, "default", "tenant_", "tenant_default"},
		{config.TenantIsolationRLSTable, "default", "tenant_", RLSTableSchemaMarker},
		{config.TenantIsolationSharedDB, "7a2f9d4e-1234-4abc-8800-000000000001", "tenant_", "tenant_7a2f9d4e_1234_4abc_8800_000000000001"},
	}
	for _, tc := range tests {
		if got := DataSchemaName(tc.tid, tc.prefix, tc.mode); got != tc.want {
			t.Errorf("DataSchemaName(%q, %q, %v) = %q, want %q", tc.tid, tc.prefix, tc.mode, got, tc.want)
		}
	}
}

func TestValidateDataSchemaReserved(t *testing.T) {
	err := ValidateDataSchema("pg_catalog", "default", "tenant_", config.TenantIsolationDedicatedDB)
	if err == nil {
		t.Fatal("expected reserved schema pg_catalog to be rejected")
	}
}

func TestValidateDataSchemaSharedMismatch(t *testing.T) {
	err := ValidateDataSchema("public", "acme", "tenant_", config.TenantIsolationSharedDB)
	if err == nil {
		t.Fatal("expected public to be rejected in shared_db for tenant acme")
	}
}

func TestResolveDataSchemaDefault(t *testing.T) {
	schema, err := ResolveDataSchema("", "acme", "tenant_", config.TenantIsolationSharedDB)
	if err != nil || schema != "tenant_acme" {
		t.Fatalf("ResolveDataSchema default: got %q err=%v", schema, err)
	}
}
