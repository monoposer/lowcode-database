package event

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/solat/lowcode-database/internal/event/delivery"
	"github.com/solat/lowcode-database/internal/infra/postgres"
	"github.com/solat/lowcode-database/internal/tenant"
	"log"
	"net/url"
	"strings"
	"time"
)

// Bus loads lc_event_sinks and POSTs JSON envelopes to each sink target_url (HTTP webhook).
type Bus struct {
	tenants   *postgres.TenantManager
	publisher Publisher
	cfg       DeliveryConfig
	dlq       *dlqWriter
	metrics   *deliveryMetrics
}

func NewBus(tenants *postgres.TenantManager, cfg DeliveryConfig) *Bus {
	if tenants == nil {
		return nil
	}
	return &Bus{
		tenants:   tenants,
		publisher: NewPublisher(),
		cfg:       cfg,
		dlq:       newDLQWriter(tenants.MetaPool()),
		metrics:   newDeliveryMetrics(cfg.MetricsEnabled),
	}
}

// Emit runs delivery asynchronously (does not block the request path).
func (b *Bus) Emit(ctx context.Context, eventType, tableID string, data map[string]any) {
	if b == nil {
		return
	}
	if !ValidEventType(eventType) {
		return
	}
	tid := strings.TrimSpace(tenant.FromContext(ctx))
	payload := map[string]any{
		"type":       eventType,
		"tenantId":   tid,
		"occurredAt": time.Now().UTC().Format(time.RFC3339Nano),
		"data":       data,
	}
	if tableID != "" {
		payload["tableId"] = tableID
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	asyncCtx := tenant.WithTenantID(context.Background(), tid)
	go b.deliver(asyncCtx, eventType, tableID, body)
}

func (b *Bus) deliver(ctx context.Context, eventType, tableID string, body []byte) {
	tid := strings.TrimSpace(tenant.FromContext(ctx))
	if tid == "" || b.tenants == nil {
		return
	}
	pool := b.tenants.MetaPool()
	rows, err := pool.Query(ctx, `
		SELECT id, target_url, COALESCE(table_filter, ''), events, headers, secret, sink_config
		FROM lc_event_sinks
		WHERE enabled = true AND tenant_id = $1
	`, tid)
	if err != nil {
		log.Printf("event: list sinks: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var sinkID, targetURL, tableFilter, secret string
		var eventsRaw, headersRaw, sinkConfigRaw []byte
		if err := rows.Scan(&sinkID, &targetURL, &tableFilter, &eventsRaw, &headersRaw, &secret, &sinkConfigRaw); err != nil {
			continue
		}
		if tableFilter != "" && tableID != "" && tableFilter != tableID {
			continue
		}
		if !eventMatches(parseEventTypes(eventsRaw), eventType) {
			continue
		}
		deliveryURL, err := ResolveDeliveryURL(SinkWebhook, targetURL, parseSinkConfig(sinkConfigRaw))
		if err != nil {
			log.Printf("event: resolve url: %v", err)
			continue
		}
		msg := delivery.Message{
			Body:    body,
			Headers: parseHeaders(headersRaw),
			Secret:  secret,
		}
		attempts, pushErr := pushWithRetry(ctx, b.publisher, deliveryURL, msg, b.cfg)
		if pushErr != nil {
			log.Printf("event: push %s after %d attempts: %v", deliveryURL, attempts, pushErr)
			if b.cfg.DLQEnabled {
				b.dlq.record(ctx, tid, sinkID, eventType, tableID, deliveryURL, DeliveryStatusDeadLetter, attempts, pushErr.Error(), body)
			}
			b.metrics.record(tid, DeliveryStatusDeadLetter, attempts)
			continue
		}
		b.metrics.record(tid, DeliveryStatusOK, attempts)
	}
}

// Publisher pushes JSON event payloads to a delivery endpoint URL.
type Publisher interface {
	Push(ctx context.Context, targetURL string, msg delivery.Message) error
}

// Router dispatches HTTP POST delivery for http and https URLs.
type Router struct {
	byScheme map[string]delivery.Driver
}

func NewPublisher() *Router {
	return &Router{
		byScheme: map[string]delivery.Driver{
			"http":  delivery.HTTP{},
			"https": delivery.HTTP{},
		},
	}
}

func (r *Router) Push(ctx context.Context, targetURL string, msg delivery.Message) error {
	if r == nil {
		return nil
	}
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return fmt.Errorf("delivery url is required")
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("parse delivery url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	drv, ok := r.byScheme[scheme]
	if !ok {
		return fmt.Errorf("unsupported delivery scheme %q", u.Scheme)
	}
	return drv.Push(ctx, u, msg)
}
