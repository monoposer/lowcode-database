package service

import (
	"net/http"
	"time"

	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/data"
	"github.com/monoposer/lowcode-database/internal/service/platform"
	"github.com/monoposer/lowcode-database/internal/service/schema"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"github.com/monoposer/lowcode-database/pkg/config"
	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
	"github.com/monoposer/lowcode-database/pkg/logger"
	"github.com/monoposer/lowcode-database/pkg/platform/cache"
)

// LowcodeService is the root facade; domain logic lives in subpackages.
type LowcodeService struct {
	*schema.Schema
	*catalog.Catalog
	*data.Data
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

func WithPGStatStatements(enabled bool) Option {
	return func(b *shared.Base) {
		b.PGStatStatements = enabled
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

func WithLimits(cfg *config.Config) Option {
	return func(b *shared.Base) {
		if cfg == nil {
			return
		}
		if cfg.MaxScanRows > 0 {
			b.MaxScanRows = int32(cfg.MaxScanRows)
		}
		if cfg.MaxBulkItems > 0 {
			b.MaxBulkItems = cfg.MaxBulkItems
		}
		if cfg.MaxExportRows > 0 {
			b.MaxExportRows = cfg.MaxExportRows
		}
		if cfg.CalcMaxRetry > 0 {
			b.CalcMaxRetry = cfg.CalcMaxRetry
		}
		if cfg.CalcTenantConcurrency > 0 {
			b.CalcTenantConcurrency = cfg.CalcTenantConcurrency
		}
		if cfg.CalcAlertQueueLen > 0 {
			b.CalcAlertQueueLen = cfg.CalcAlertQueueLen
		}
		if cfg.IndexBackfillTimeoutSec > 0 {
			b.IndexBackfillTimeout = time.Duration(cfg.IndexBackfillTimeoutSec) * time.Second
		}
		b.DDLConfirmRequired = cfg.DDLConfirmRequired
	}
}

func WithHTTPMiddleware(mw func(http.Handler) http.Handler) Option {
	return func(b *shared.Base) {
		b.HTTPMiddleware = mw
	}
}

func WithEventBus(bus event.Bus) Option {
	return func(b *shared.Base) {
		b.EventBus = bus
	}
}

func NewLowcodeService(tenants *postgres.TenantManager, maxRow int, opts ...Option) *LowcodeService {
	base := shared.NewBase(tenants, maxRow)
	for _, opt := range opts {
		opt(base)
	}
	return &LowcodeService{
		schema.New(base),
		catalog.New(base),
		data.New(base),
		platform.New(base),
	}
}
