package event

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const defaultStream = "lc:events"

// RedisBus publishes to a Redis Stream and consumes via a consumer group.
type RedisBus struct {
	rdb    *redis.Client
	stream string
}

func NewRedisBus(rdb *redis.Client, stream string) *RedisBus {
	if stream == "" {
		stream = defaultStream
	}
	return &RedisBus{rdb: rdb, stream: stream}
}

func (b *RedisBus) Publish(ctx context.Context, e Envelope) error {
	if b == nil || b.rdb == nil {
		return fmt.Errorf("redis event bus is not configured")
	}
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return b.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: b.stream,
		Values: map[string]any{"payload": string(raw)},
	}).Err()
}

func (b *RedisBus) Subscribe(ctx context.Context, group, consumer string, h Handler) error {
	if b == nil || b.rdb == nil {
		return fmt.Errorf("redis event bus is not configured")
	}
	if group == "" {
		group = "webhooks"
	}
	if consumer == "" {
		consumer = "c-" + uuid.NewString()[:8]
	}
	_ = b.rdb.XGroupCreateMkStream(ctx, b.stream, group, "0").Err()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		res, err := b.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{b.stream, ">"},
			Count:    16,
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if err == redis.Nil || strings.Contains(err.Error(), "NOGROUP") {
				_ = b.rdb.XGroupCreateMkStream(ctx, b.stream, group, "0").Err()
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}
		for _, stream := range res {
			for _, msg := range stream.Messages {
				env, perr := parseStreamMessage(msg)
				if perr != nil {
					_ = b.rdb.XAck(ctx, b.stream, group, msg.ID).Err()
					continue
				}
				if h != nil {
					if herr := h(ctx, env); herr != nil {
						continue
					}
				}
				_ = b.rdb.XAck(ctx, b.stream, group, msg.ID).Err()
			}
		}
	}
}

func parseStreamMessage(msg redis.XMessage) (Envelope, error) {
	raw, _ := msg.Values["payload"].(string)
	if raw == "" {
		return Envelope{}, fmt.Errorf("empty payload")
	}
	var e Envelope
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return Envelope{}, err
	}
	if e.ID == "" {
		e.ID = msg.ID
	}
	return e, nil
}

func (b *RedisBus) Close() error {
	return nil
}
