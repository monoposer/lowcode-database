package cache

import (
	"context"
	"encoding/json"
	"time"
)

// MetaCache caches metadata blobs (data source / column specs).
type MetaCache interface {
	Get(ctx context.Context, key string, dest any) (found bool, err error)
	Set(ctx context.Context, key string, val any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// Noop implements MetaCache with no storage.
type Noop struct{}

func (Noop) Get(context.Context, string, any) (bool, error) { return false, nil }
func (Noop) Set(context.Context, string, any, time.Duration) error { return nil }
func (Noop) Delete(context.Context, ...string) error             { return nil }

func encodeJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func decodeJSON(data []byte, dest any) error {
	return json.Unmarshal(data, dest)
}
