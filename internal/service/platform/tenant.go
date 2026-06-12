package platform

import (
	"context"
	"fmt"
	"github.com/solat/lowcode-database/internal/apiv1/platform"
	apiv1schema "github.com/solat/lowcode-database/internal/apiv1/schema"
	"github.com/solat/lowcode-database/internal/columntype"
	"strings"
)

// -------- Tenant --------

func (s *Platform) CreateTenant(ctx context.Context, req *platform.CreateTenantRequest) (*platform.CreateTenantResponse, error) {
	id := strings.TrimSpace(req.Id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	if err := s.B.Tenants.CreateTenant(ctx, id, req.DisplayName, req.DataDsn, req.ReadDsn, req.ReadOnly, req.PoolMaxConns, req.CreateDatabase); err != nil {
		return nil, fmt.Errorf("create tenant %s: %w", id, err)
	}
	return &platform.CreateTenantResponse{Id: id}, nil
}

// ListTypes returns built-in column types (hard-coded, not from DB).
func (s *Platform) ListTypes(ctx context.Context, _ *platform.ListTypesRequest) (*platform.ListTypesResponse, error) {
	_ = ctx
	var res platform.ListTypesResponse
	for _, t := range columntype.List() {
		res.Types = append(res.Types, &apiv1schema.Type{
			Id:     t.ID,
			Name:   t.Name,
			PgType: t.PgType,
			Config: t.Config,
		})
	}
	return &res, nil
}
