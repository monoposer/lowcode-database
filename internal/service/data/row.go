package data

import (
	"context"

)

// CreateRow / UpdateRow / DeleteRow live in vr_rows.go.

// ListRows delegates to QueryRows for filter/sort/pagination support.
func (s *Data) ListRows(ctx context.Context, req *ListRowsRequest) (*ListRowsResponse, error) {
	qresp, err := s.QueryRows(ctx, &QueryRowsRequest{
		TableName:     req.TableName,
		PageSize:    req.PageSize,
		PageToken:   req.PageToken,
		Consistency: req.Consistency,
	})
	if err != nil {
		return nil, err
	}
	return &ListRowsResponse{
		Rows:          qresp.Rows,
		NextPageToken: qresp.NextPageToken,
	}, nil
}
