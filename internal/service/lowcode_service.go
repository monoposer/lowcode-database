package service

import (
	"context"
	"time"

	"github.com/solat/lowcode-database/internal/cache"
	"github.com/solat/lowcode-database/internal/db"
	"github.com/solat/lowcode-database/internal/logger"
	"github.com/solat/lowcode-database/internal/metrics"
	"github.com/solat/lowcode-database/internal/webhook"
)

type LowcodeService struct {
	tenants *db.TenantManager
	Hooks   *webhook.Dispatcher

	maxRow int32

	cache              cache.MetaCache
	cacheTTL           time.Duration
	dsMetrics          metrics.DataSourceMetrics
	log                *logger.Logger
	slowQueryThreshold time.Duration
}

type Option func(*LowcodeService)

func WithCache(c cache.MetaCache, ttl time.Duration) Option {
	return func(s *LowcodeService) {
		if c != nil {
			s.cache = c
		}
		if ttl > 0 {
			s.cacheTTL = ttl
		}
	}
}

func WithMetrics(m metrics.DataSourceMetrics) Option {
	return func(s *LowcodeService) {
		if m != nil {
			s.dsMetrics = m
		}
	}
}

func WithLogger(l *logger.Logger, slowQueryThreshold time.Duration) Option {
	return func(s *LowcodeService) {
		if l != nil {
			s.log = l
		}
		if slowQueryThreshold > 0 {
			s.slowQueryThreshold = slowQueryThreshold
		}
	}
}

func NewLowcodeService(tenants *db.TenantManager, maxRow int, hooks *webhook.Dispatcher, opts ...Option) *LowcodeService {
	s := &LowcodeService{
		tenants:            tenants,
		Hooks:              hooks,
		cache:              cache.Noop{},
		cacheTTL:           5 * time.Minute,
		dsMetrics:          metrics.Noop{},
		log:                logger.Default(),
		slowQueryThreshold: 500 * time.Millisecond,
	}
	if maxRow > 0 {
		s.maxRow = int32(maxRow)
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// DataSourceQueryStats returns rolling average latency for a data source.
func (s *LowcodeService) DataSourceQueryStats(ctx context.Context, dataSourceID string) (metrics.QueryStats, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return metrics.QueryStats{}, err
	}
	return s.dsMetrics.Stats(ctx, tid, dataSourceID)
}
