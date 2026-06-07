package platform

import (
	"context"
	"fmt"
	"strings"

	"github.com/monoposer/lowcode-database/internal/apiv1/platform"
	"github.com/monoposer/lowcode-database/internal/infra/postgres"
)

// CreateTenant registers a tenant and provisions data storage per isolation mode.
func (s *Platform) CreateTenant(ctx context.Context, req *platform.CreateTenantRequest) (*platform.CreateTenantResponse, error) {
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
	return &platform.CreateTenantResponse{Id: id, RecordStore: store}, nil
}

func (s *Platform) UpdateTenant(ctx context.Context, id string, req *platform.UpdateTenantRequest) (*platform.UpdateTenantResponse, error) {
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
	return &platform.UpdateTenantResponse{Id: id}, nil
}
