package platform

import (
	"context"
	"github.com/monoposer/lowcode-database/pkg/platform/metrics"
)

func (s *Platform) ListPGStatStatements(ctx context.Context, req *ListPGStatStatementsRequest) (*ListPGStatStatementsResponse, error) {
	if s.B == nil || !s.B.PGStatStatements {
		return &ListPGStatStatementsResponse{Enabled: false, Statements: []PGStatStatement{}}, nil
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
	out := make([]PGStatStatement, 0, len(rows))
	for _, r := range rows {
		out = append(out, PGStatStatement{
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
	return &ListPGStatStatementsResponse{Enabled: true, Statements: out}, nil
}
