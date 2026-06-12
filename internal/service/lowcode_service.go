package service

import (
	"context"
	"time"

	"github.com/solat/lowcode-database/internal/event"
	"github.com/solat/lowcode-database/internal/infra/postgres"
	"github.com/solat/lowcode-database/internal/logger"
	"github.com/solat/lowcode-database/internal/platform/cache"
	"github.com/solat/lowcode-database/internal/platform/metrics"
	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/data"
	"github.com/solat/lowcode-database/internal/service/graph"
	"github.com/solat/lowcode-database/internal/service/platform"
	"github.com/solat/lowcode-database/internal/service/schema"
	"github.com/solat/lowcode-database/internal/service/shared"
	"github.com/solat/lowcode-database/internal/telemetry"
)

// LowcodeService is the root facade; domain logic lives in subpackages.
type LowcodeService struct {
	*schema.Schema
	*catalog.Catalog
	*data.Data
	*graph.Graph
	*platform.Platform
}

type Option func(*shared.Base)

func WithCache(c cache.MetaCache, ttl time.Duration) Option {
	return func(b *shared.Base) {
		if c != nil {
			b.Cache = c
		}
		if ttl > 0 {
			b.CacheTTL = ttl
		}
	}
}

func WithMetrics(m metrics.DataSourceMetrics) Option {
	return func(b *shared.Base) {
		if m != nil {
			b.DSMetrics = m
		}
	}
}

func WithLogger(l *logger.Logger, slowQueryThreshold time.Duration) Option {
	return func(b *shared.Base) {
		if l != nil {
			b.Log = l
		}
		if slowQueryThreshold > 0 {
			b.SlowQueryThreshold = slowQueryThreshold
		}
	}
}

func WithLogSQL(enabled bool) Option {
	return func(b *shared.Base) {
		b.LogSQL = enabled
	}
}

func WithTelemetry(p telemetry.Provider) Option {
	return func(b *shared.Base) {
		if p != nil {
			b.Telemetry = p
		}
	}
}

func NewLowcodeService(tenants *postgres.TenantManager, maxRow int, bus *event.Bus, opts ...Option) *LowcodeService {
	base := shared.NewBase(tenants, maxRow, bus)
	for _, opt := range opts {
		opt(base)
	}
	return &LowcodeService{
		schema.New(base),
		catalog.New(base),
		data.New(base),
		graph.New(base),
		platform.New(base),
	}
}

// DataSourceQueryStats returns rolling average latency for a data source.
func (s *LowcodeService) DataSourceQueryStats(ctx context.Context, dataSourceID string) (metrics.QueryStats, error) {
	tid, err := s.Schema.B.TenantID(ctx)
	if err != nil {
		return metrics.QueryStats{}, err
	}
	return s.Schema.B.DSMetrics.Stats(ctx, tid, dataSourceID)
}
