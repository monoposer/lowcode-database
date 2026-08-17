package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
	"github.com/monoposer/lowcode-database/pkg/platform/authn"
)

const (
	defaultPublicBaseName  = "public"
	defaultPublicBaseLabel = "Public"
	defaultAPIKeyName      = "default"
)

// CreateTenant registers a tenant, provisions data tables, then seeds a public base and default API key.
func (s *Platform) CreateTenant(ctx context.Context, req *CreateTenantRequest) (*CreateTenantResponse, error) {
	id := strings.TrimSpace(req.Id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	writeDSN := strings.TrimSpace(req.DataDsnWrite)
	if writeDSN == "" {
		writeDSN = strings.TrimSpace(req.DataDsn)
	}
	store, err := postgres.ParseRecordStore(req.RecordStore)
	if err != nil {
		return nil, err
	}
	created, err := s.B.Tenants.CreateTenantFull(ctx, id, req.DisplayName, writeDSN, req.DataDsnReads, req.PoolMaxConns, store)
	if err != nil {
		return nil, fmt.Errorf("create tenant %s: %w", id, err)
	}
	out := &CreateTenantResponse{Id: id, RecordStore: store}
	if !created {
		return out, nil
	}

	baseID := "base_" + id
	if _, err := s.B.Tenants.CreateBase(ctx, id, baseID, defaultPublicBaseName, defaultPublicBaseLabel); err != nil {
		return nil, fmt.Errorf("create public base for tenant %s: %w", id, err)
	}
	out.Base = &BaseDTO{
		BaseID: baseID, TenantID: id, Name: defaultPublicBaseName, Label: defaultPublicBaseLabel, Status: "active",
	}

	plain, hash, prefix, err := authn.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate api key for tenant %s: %w", id, err)
	}
	var ak APIKey
	var createdAt, updatedAt time.Time
	err = s.B.Tenants.MetaPool().QueryRow(ctx, `
		INSERT INTO lc_api_keys (tenant_id, name, key_hash, key_prefix, rate_limit_rps)
		VALUES ($1, $2, $3, $4, 0)
		RETURNING id, name, key_prefix, enabled, rate_limit_rps, created_at, updated_at
	`, id, defaultAPIKeyName, hash, prefix).Scan(
		&ak.Id, &ak.Name, &ak.KeyPrefix, &ak.Enabled, &ak.RateLimitRps, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create api key for tenant %s: %w", id, err)
	}
	ak.CreatedAt = createdAt
	ak.UpdatedAt = updatedAt
	out.ApiKey = &ak
	out.Key = plain
	return out, nil
}

func (s *Platform) UpdateTenant(ctx context.Context, id string, req *UpdateTenantRequest) (*UpdateTenantResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	if req == nil {
		return nil, fmt.Errorf("body is required")
	}
	writeDSN := strings.TrimSpace(req.DataDsnWrite)
	if writeDSN == "" {
		writeDSN = strings.TrimSpace(req.DataDsn)
	}
	updateReads := req.DataDsnReads != nil
	if err := s.B.Tenants.UpdateTenantDSNs(ctx, id, writeDSN, req.DataDsnReads, updateReads); err != nil {
		return nil, err
	}
	return &UpdateTenantResponse{Id: id}, nil
}

type TenantDTO struct {
	TenantID         string `json:"tenantId"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	ReadReplicaCount int    `json:"readReplicaCount,omitempty"`
	RecordStore      string `json:"recordStore,omitempty"`
}

type ListTenantsResponse struct {
	Tenants []TenantDTO `json:"tenants"`
}

func (s *Platform) ListTenants(ctx context.Context) (*ListTenantsResponse, error) {
	list, err := s.B.Tenants.ListTenants(ctx, "")
	if err != nil {
		return nil, err
	}
	out := make([]TenantDTO, 0, len(list))
	for _, t := range list {
		out = append(out, tenantDTO(t))
	}
	return &ListTenantsResponse{Tenants: out}, nil
}

func tenantDTO(t postgres.TenantInfo) TenantDTO {
	return TenantDTO{
		TenantID: t.TenantID, Name: t.Name, Status: t.Status, ReadReplicaCount: t.ReadReplicaCount,
		RecordStore: t.RecordStore,
	}
}
