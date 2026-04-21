package metrics

import (
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/solat/lowcode-database/internal/config"
)

// New returns a DataSourceMetrics backend from config.
func New(cfg *config.Config, rdb *redis.Client) DataSourceMetrics {
	switch strings.ToLower(cfg.MetricsBackend) {
	case "redis":
		if rdb == nil {
			return Noop{}
		}
		return NewRedis(rdb, cfg.MetricsWindowSize)
	case "prometheus":
		return NewPrometheus(cfg.MetricsWindowSize)
	default:
		return Noop{}
	}
}
