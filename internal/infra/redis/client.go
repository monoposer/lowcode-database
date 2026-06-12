package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/solat/lowcode-database/internal/config"
)

// Open connects to Redis when REDIS_URL is set; otherwise returns (nil, nil).
func Open(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	if cfg.RedisURL == "" {
		return nil, nil
	}
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	client := redis.NewClient(opt)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return client, nil
}
