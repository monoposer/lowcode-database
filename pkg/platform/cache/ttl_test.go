package cache

import (
	"context"
	"testing"
	"time"
)

type mem struct {
	ttl map[string]time.Duration
	val map[string]any
}

func newMem() *mem {
	return &mem{ttl: map[string]time.Duration{}, val: map[string]any{}}
}

func (m *mem) Get(_ context.Context, key string, dest any) (bool, error) {
	v, ok := m.val[key]
	if !ok {
		return false, nil
	}
	if p, ok := dest.(*string); ok {
		*p, _ = v.(string)
	}
	return true, nil
}

func (m *mem) Set(_ context.Context, key string, val any, ttl time.Duration) error {
	m.val[key] = val
	m.ttl[key] = ttl
	return nil
}

func (m *mem) Delete(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.val, k)
		delete(m.ttl, k)
	}
	return nil
}

func TestDefaultTTLFallback(t *testing.T) {
	inner := newMem()
	c := WithDefaultTTL(inner, 30*time.Second)
	if err := c.Set(context.Background(), "k", "v", 0); err != nil {
		t.Fatal(err)
	}
	if inner.ttl["k"] != 30*time.Second {
		t.Fatalf("ttl=%v", inner.ttl["k"])
	}
}
