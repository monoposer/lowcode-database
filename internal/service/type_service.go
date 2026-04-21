package service

import (
	"context"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/columntype"
)

// ListTypes returns built-in column types (hard-coded, not from DB).
func (s *LowcodeService) ListTypes(ctx context.Context, _ *apiv1.ListTypesRequest) (*apiv1.ListTypesResponse, error) {
	_ = ctx
	var res apiv1.ListTypesResponse
	for _, t := range columntype.List() {
		res.Types = append(res.Types, &apiv1.Type{
			Id:     t.ID,
			Name:   t.Name,
			PgType: t.PgType,
			Config: t.Config,
		})
	}
	return &res, nil
}
