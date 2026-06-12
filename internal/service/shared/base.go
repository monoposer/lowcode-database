package shared

import (
	"time"

	"github.com/solat/lowcode-database/internal/event"
	"github.com/solat/lowcode-database/internal/infra/postgres"
	"github.com/solat/lowcode-database/internal/logger"
	"github.com/solat/lowcode-database/internal/platform/cache"
	"github.com/solat/lowcode-database/internal/platform/metrics"
	"github.com/solat/lowcode-database/internal/telemetry"
)

// Base holds shared dependencies for all domain services.
type Base struct {
	Tenants            *postgres.TenantManager
	Events             *event.Bus
	Telemetry          telemetry.Provider
	MaxRow             int32
	Cache              cache.MetaCache
	CacheTTL           time.Duration
	DSMetrics          metrics.DataSourceMetrics
	Log                *logger.Logger
	SlowQueryThreshold time.Duration
	LogSQL             bool
}

func NewBase(tenants *postgres.TenantManager, maxRow int, events *event.Bus) *Base {
	b := &Base{
		Tenants:            tenants,
		Events:             events,
		Telemetry:          telemetry.Noop{},
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
