package platform

import (
	"context"
	"strings"

	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
)

type BaseDTO struct {
	BaseID   string `json:"baseId"`
	TenantID string `json:"tenantId"`
	Name     string `json:"name"`
	Label    string `json:"label"`
	Status   string `json:"status"`
}

type ListBasesResponse struct {
	Bases []BaseDTO `json:"bases"`
}

type CreateBaseRequest struct {
	BaseID string `json:"baseId"`
	Name   string `json:"name"`
	Label  string `json:"label"`
}

type CreateBaseResponse struct {
	Base BaseDTO `json:"base"`
}

func (s *Platform) ListBases(ctx context.Context) (*ListBasesResponse, error) {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	list, err := s.B.Tenants.ListBases(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]BaseDTO, 0, len(list))
	for _, b := range list {
		out = append(out, baseDTO(b))
	}
	return &ListBasesResponse{Bases: out}, nil
}

func (s *Platform) CreateBase(ctx context.Context, req *CreateBaseRequest) (*CreateBaseResponse, error) {
	tenantID, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil {
		req = &CreateBaseRequest{}
	}
	baseID, err := s.B.Tenants.CreateBase(ctx, tenantID, req.BaseID, req.Name, req.Label)
	if err != nil {
		return nil, err
	}
	list, err := s.B.Tenants.ListBases(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, b := range list {
		if b.BaseID == baseID {
			return &CreateBaseResponse{Base: baseDTO(b)}, nil
		}
	}
	return &CreateBaseResponse{Base: BaseDTO{
		BaseID: baseID, TenantID: tenantID, Name: strings.TrimSpace(req.Name), Label: strings.TrimSpace(req.Label), Status: "active",
	}}, nil
}

func baseDTO(b postgres.BaseInfo) BaseDTO {
	return BaseDTO{
		BaseID: b.BaseID, TenantID: b.TenantID, Name: b.Name, Label: b.Label, Status: b.Status,
	}
}
