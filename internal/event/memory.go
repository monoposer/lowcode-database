package event

import (
	"context"
	"sync"
)

// MemoryBus is an in-process fan-out bus for single-instance development.
type MemoryBus struct {
	mu     sync.RWMutex
	subs   []Handler
	closed bool
}

func NewMemoryBus() *MemoryBus {
	return &MemoryBus{}
}

func (b *MemoryBus) Publish(ctx context.Context, e Envelope) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil
	}
	subs := append([]Handler(nil), b.subs...)
	b.mu.RUnlock()
	for _, h := range subs {
		if err := ctx.Err(); err != nil {
			return err
		}
		_ = h(ctx, e)
	}
	return nil
}

func (b *MemoryBus) Subscribe(ctx context.Context, _, _ string, h Handler) error {
	if h == nil {
		<-ctx.Done()
		return ctx.Err()
	}
	b.mu.Lock()
	b.subs = append(b.subs, h)
	b.mu.Unlock()
	<-ctx.Done()
	return ctx.Err()
}

func (b *MemoryBus) Close() error {
	b.mu.Lock()
	b.closed = true
	b.subs = nil
	b.mu.Unlock()
	return nil
}
