package platform

import (
	"context"
	"fmt"
	"strings"
	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
)

// CreateTenant registers a tenant and provisions data storage per isolation mode.
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
	if err := s.B.Tenants.CreateTenantFull(ctx, id, req.DisplayName, writeDSN, req.DataDsnReads, req.PoolMaxConns, store); err != nil {
		return nil, fmt.Errorf("create tenant %s: %w", id, err)
	}
	return &CreateTenantResponse{Id: id, RecordStore: store}, nil
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
