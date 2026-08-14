package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/monoposer/lowcode-database/pkg/config"
	"github.com/redis/go-redis/v9"
	"time"
)

// MetaCache caches metadata blobs (query / column specs).
type MetaCache interface {
	Get(ctx context.Context, key string, dest any) (found bool, err error)
	Set(ctx context.Context, key string, val any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// Noop implements MetaCache with no storage.
type Noop struct{}

func (Noop) Get(context.Context, string, any) (bool, error)        { return false, nil }
func (Noop) Set(context.Context, string, any, time.Duration) error { return nil }
func (Noop) Delete(context.Context, ...string) error               { return nil }

func encodeJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func decodeJSON(data []byte, dest any) error {
	return json.Unmarshal(data, dest)
}

// New returns a MetaCache from config (Redis when enabled, otherwise Noop).
func New(cfg *config.Config, rdb *redis.Client) MetaCache {
	if !cfg.CacheEnabled || rdb == nil {
		return Noop{}
	}
	ttl := time.Duration(cfg.CacheTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return WithDefaultTTL(NewRedis(rdb), ttl)
}

// WithDefaultTTL wraps a cache so Set never stores keys without an expiry.
type ttlCache struct {
	inner MetaCache
	ttl   time.Duration
}

func WithDefaultTTL(inner MetaCache, ttl time.Duration) MetaCache {
	if inner == nil {
		return Noop{}
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ttlCache{inner: inner, ttl: ttl}
}

func (c *ttlCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	return c.inner.Get(ctx, key, dest)
}

func (c *ttlCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.ttl
	}
	return c.inner.Set(ctx, key, val, ttl)
}

func (c *ttlCache) Delete(ctx context.Context, keys ...string) error {
	return c.inner.Delete(ctx, keys...)
}

// RawGetter is implemented by Redis for cache inspect.
type RawGetter interface {
	GetRaw(ctx context.Context, key string) ([]byte, bool, error)
}

// Redis stores JSON-encoded metadata in Redis.
type Redis struct {
	client *redis.Client
}

func NewRedis(client *redis.Client) *Redis {
	return &Redis{client: client}
}

func (c *Redis) Get(ctx context.Context, key string, dest any) (bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis get %q: %w", key, err)
	}
	if err := decodeJSON(data, dest); err != nil {
		return false, fmt.Errorf("redis decode %q: %w", key, err)
	}
	return true, nil
}

func (c *Redis) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	data, err := encodeJSON(val)
	if err != nil {
		return err
	}
	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %q: %w", key, err)
	}
	return nil
}

func (c *Redis) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}
	return nil
}

func (c *Redis) GetRaw(ctx context.Context, key string) ([]byte, bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get %q: %w", key, err)
	}
	return data, true, nil
}

func (c *ttlCache) GetRaw(ctx context.Context, key string) ([]byte, bool, error) {
	if g, ok := c.inner.(RawGetter); ok {
		return g.GetRaw(ctx, key)
	}
	return nil, false, nil
}
