package sink

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func deliverRedis(ctx context.Context, cfg map[string]any, body []byte) {
	url := configString(cfg, "url")
	if url == "" {
		return
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("sink redis parse url: %v", err)
		return
	}
	client := redis.NewClient(opt)
	defer client.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := client.Ping(pingCtx).Err(); err != nil {
		cancel()
		log.Printf("sink redis ping: %v", err)
		return
	}
	cancel()

	mode := configString(cfg, "mode")
	if mode == "" {
		mode = "stream"
	}
	switch mode {
	case "stream":
		stream := configString(cfg, "stream")
		if stream == "" {
			return
		}
		if err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			Values: map[string]interface{}{"payload": string(body)},
		}).Err(); err != nil {
			log.Printf("sink redis XADD %s: %v", stream, err)
		}
	case "pubsub":
		channel := configString(cfg, "channel")
		if channel == "" {
			return
		}
		if err := client.Publish(ctx, channel, body).Err(); err != nil {
			log.Printf("sink redis PUBLISH %s: %v", channel, err)
		}
	default:
		log.Printf("sink redis: unknown mode %q", mode)
	}
}
