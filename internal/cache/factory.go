package cache

import (
	"github.com/redis/go-redis/v9"

	"github.com/solat/lowcode-database/internal/config"
)

// New returns a MetaCache from config (Redis when enabled, otherwise Noop).
func New(cfg *config.Config, rdb *redis.Client) MetaCache {
	if !cfg.CacheEnabled || rdb == nil {
		return Noop{}
	}
	return NewRedis(rdb)
}
