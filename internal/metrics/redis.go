package metrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis stores last N query durations per data source in a Redis list.
type Redis struct {
	client *redis.Client
	window int
}

func NewRedis(client *redis.Client, window int) *Redis {
	if window <= 0 {
		window = 100
	}
	return &Redis{client: client, window: window}
}

func (m *Redis) key(tenantID, dataSourceID string) string {
	return fmt.Sprintf("lc:metrics:ds:%s:%s", tenantID, dataSourceID)
}

func (m *Redis) Record(ctx context.Context, tenantID, dataSourceID string, duration time.Duration, _ error) {
	key := m.key(tenantID, dataSourceID)
	us := strconv.FormatInt(duration.Microseconds(), 10)
	pipe := m.client.Pipeline()
	pipe.LPush(ctx, key, us)
	pipe.LTrim(ctx, key, 0, int64(m.window-1))
	pipe.Expire(ctx, key, 7*24*time.Hour)
	_, _ = pipe.Exec(ctx)
}

func (m *Redis) Stats(ctx context.Context, tenantID, dataSourceID string) (QueryStats, error) {
	vals, err := m.client.LRange(ctx, m.key(tenantID, dataSourceID), 0, int64(m.window-1)).Result()
	if err != nil {
		return QueryStats{}, fmt.Errorf("redis lrange: %w", err)
	}
	if len(vals) == 0 {
		return QueryStats{}, nil
	}
	var sum int64
	for _, v := range vals {
		us, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		sum += us
	}
	return QueryStats{
		AvgDuration: time.Duration(sum/int64(len(vals))) * time.Microsecond,
		Count:       len(vals),
	}, nil
}
