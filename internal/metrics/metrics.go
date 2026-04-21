package metrics

import (
	"context"
	"time"
)

// QueryStats holds rolling-window stats for a data source.
type QueryStats struct {
	AvgDuration time.Duration
	Count       int // samples in window (≤ window size)
}

// DataSourceMetrics records data source query latency.
type DataSourceMetrics interface {
	Record(ctx context.Context, tenantID, dataSourceID string, duration time.Duration, err error)
	Stats(ctx context.Context, tenantID, dataSourceID string) (QueryStats, error)
}

// Noop discards metrics.
type Noop struct{}

func (Noop) Record(context.Context, string, string, time.Duration, error) {}
func (Noop) Stats(context.Context, string, string) (QueryStats, error) {
	return QueryStats{}, nil
}
