package shared

import (
	"time"

	"github.com/solat/lowcode-database/internal/cache"
	"github.com/solat/lowcode-database/internal/db"
	"github.com/solat/lowcode-database/internal/logger"
	"github.com/solat/lowcode-database/internal/metrics"
	"github.com/solat/lowcode-database/internal/webhook"
)

// Base holds shared dependencies for all domain services.
type Base struct {
	Tenants            *db.TenantManager
	Hooks              *webhook.Dispatcher
	MaxRow             int32
	Cache              cache.MetaCache
	CacheTTL           time.Duration
	DSMetrics          metrics.DataSourceMetrics
	Log                *logger.Logger
	SlowQueryThreshold time.Duration
	LogSQL             bool
}

func NewBase(tenants *db.TenantManager, maxRow int, hooks *webhook.Dispatcher) *Base {
	b := &Base{
		Tenants:            tenants,
		Hooks:              hooks,
		Cache:              cache.Noop{},
		CacheTTL:           5 * time.Minute,
		DSMetrics:          metrics.Noop{},
		Log:                logger.Default(),
		SlowQueryThreshold: 500 * time.Millisecond,
	}
	if maxRow > 0 {
		b.MaxRow = int32(maxRow)
	}
	return b
}
