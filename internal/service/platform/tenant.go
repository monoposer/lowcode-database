package platform

import (
	"context"
	"fmt"
	"strings"

	"github.com/solat/lowcode-database/internal/apiv1"
)

// -------- Tenant --------

func (s *Platform) CreateTenant(ctx context.Context, req *apiv1.CreateTenantRequest) (*apiv1.CreateTenantResponse, error) {
	id := strings.TrimSpace(req.Id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	if err := s.B.Tenants.CreateTenant(ctx, id, req.DisplayName, req.DataDsn, req.CreateDatabase); err != nil {
		return nil, fmt.Errorf("create tenant %s: %w", id, err)
	}
	return &apiv1.CreateTenantResponse{Id: id}, nil
}
