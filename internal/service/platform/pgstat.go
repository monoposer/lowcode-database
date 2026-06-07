package platform

import (
	"context"

	apiv1platform "github.com/monoposer/lowcode-database/internal/apiv1/platform"
	"github.com/monoposer/lowcode-database/internal/platform/metrics"
)

func (s *Platform) ListPGStatStatements(ctx context.Context, req *apiv1platform.ListPGStatStatementsRequest) (*apiv1platform.ListPGStatStatementsResponse, error) {
	if s.B == nil || !s.B.PGStatStatements {
		return &apiv1platform.ListPGStatStatementsResponse{Enabled: false, Statements: []apiv1platform.PGStatStatement{}}, nil
	}
	pool, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	limit := 0
	if req != nil {
		limit = req.Limit
	}
	rows, err := metrics.List(ctx, pool, limit)
	if err != nil {
		return nil, err
	}
	out := make([]apiv1platform.PGStatStatement, 0, len(rows))
	for _, r := range rows {
		out = append(out, apiv1platform.PGStatStatement{
			QueryID:        r.QueryID,
			Query:          r.Query,
			Calls:          r.Calls,
			TotalExecTime:  r.TotalExecTime,
			MeanExecTime:   r.MeanExecTime,
			MinExecTime:    r.MinExecTime,
			MaxExecTime:    r.MaxExecTime,
			Rows:           r.Rows,
			SharedBlksHit:  r.SharedBlksHit,
			SharedBlksRead: r.SharedBlksRead,
		})
	}
	return &apiv1platform.ListPGStatStatementsResponse{Enabled: true, Statements: out}, nil
}
