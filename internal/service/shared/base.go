package shared

import (
	"net/http"
	"time"

	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
	"github.com/monoposer/lowcode-database/pkg/logger"
	"github.com/monoposer/lowcode-database/pkg/platform/cache"
)

// Base holds shared dependencies for all domain services.
type Base struct {
	Tenants            *postgres.TenantManager
	MaxRow             int32
	Cache              cache.MetaCache
	CacheTTL           time.Duration
	Log                *logger.Logger
	SlowQueryThreshold time.Duration
	// PGStatStatements enables GET /v1/admin/pg-stat-statements (pg_stat_statements).
	PGStatStatements bool

	MaxScanRows           int32
	MaxBulkItems          int
	MaxExportRows         int
	CalcMaxRetry          int
	CalcTenantConcurrency int
	CalcAlertQueueLen     int
	IndexBackfillTimeout  time.Duration
	DDLConfirmRequired    bool
	HTTPMiddleware        func(http.Handler) http.Handler
	EventBus              event.Bus
}

func NewBase(tenants *postgres.TenantManager, maxRow int) *Base {
	b := &Base{
		Tenants:               tenants,
		Cache:                 cache.Noop{},
		CacheTTL:              5 * time.Minute,
		Log:                   logger.Default(),
		SlowQueryThreshold:    500 * time.Millisecond,
		MaxBulkItems:          500,
		MaxExportRows:         100000,
		CalcMaxRetry:          8,
		CalcTenantConcurrency: 8,
	}
	if maxRow > 0 {
		b.MaxRow = int32(maxRow)
	}
	return b
}
