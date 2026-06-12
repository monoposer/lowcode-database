package event

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/solat/lowcode-database/internal/event/delivery"
	"time"
)

func pushWithRetry(ctx context.Context, pub Publisher, url string, msg delivery.Message, cfg DeliveryConfig) (attempts int, err error) {
	max := cfg.RetryMax
	if max <= 0 {
		max = 1
	}
	backoff := cfg.RetryInitial
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}
	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		attempts = attempt
		lastErr = pub.Push(ctx, url, msg)
		if lastErr == nil {
			return attempts, nil
		}
		if attempt < max {
			select {
			case <-ctx.Done():
				return attempts, ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}
	return attempts, lastErr
}

const (
	DeliveryStatusOK         = "ok"
	DeliveryStatusDeadLetter = "dead_letter"
)

type dlqWriter struct {
	pool *pgxpool.Pool
}

func newDLQWriter(pool *pgxpool.Pool) *dlqWriter {
	if pool == nil {
		return nil
	}
	return &dlqWriter{pool: pool}
}

func (w *dlqWriter) record(ctx context.Context, tenantID, sinkID, eventType, tableID, targetURL, status string, attempts int, lastErr string, payload []byte) {
	if w == nil || w.pool == nil || status != DeliveryStatusDeadLetter {
		return
	}
	var payloadJSON any
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &payloadJSON)
	}
	if payloadJSON == nil {
		payloadJSON = map[string]any{}
	}
	b, _ := json.Marshal(payloadJSON)
	var sinkUUID any
	if sinkID != "" {
		sinkUUID = sinkID
	}
	_, _ = w.pool.Exec(ctx, `
		INSERT INTO lc_event_delivery_log
			(tenant_id, sink_id, event_type, table_id, target_url, attempts, status, last_error, payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
	`, tenantID, sinkUUID, eventType, tableID, targetURL, attempts, status, lastErr, string(b))
}
