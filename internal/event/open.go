package event

import (
	"context"
	"os"

	"github.com/redis/go-redis/v9"

	"github.com/monoposer/lowcode-database/pkg/config"
)

// Open returns a Redis Stream bus when EVENT_BUS=redis and a client is present,
// otherwise an in-process MemoryBus.
func Open(cfg *config.Config, rdb *redis.Client) Bus {
	if cfg != nil && cfg.EventBus == "redis" && rdb != nil {
		return NewRedisBus(rdb, cfg.EventStreamKey)
	}
	return NewMemoryBus()
}

// StartWebhookDispatcher consumes the bus and POSTs matching lc_event_webhooks.
func StartWebhookDispatcher(ctx context.Context, bus Bus, store WebhookStore) {
	if bus == nil || store == nil {
		return
	}
	consumer, _ := os.Hostname()
	if consumer == "" {
		consumer = "webhook"
	}
	go func() {
		_ = bus.Subscribe(ctx, "webhooks", consumer, DeliverWebhooks(store, nil))
	}()
}
