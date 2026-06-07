package sink

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/solat/lowcode-database/internal/db"
	"github.com/solat/lowcode-database/internal/tenant"
)

// Dispatcher loads lc_webhooks and delivers JSON payloads to configured sinks.
// Payload shape (NocoDB-like): top-level "type", "tableId", "data".
type Dispatcher struct {
	tenants *db.TenantManager
	client  *http.Client
}

func NewDispatcher(tenants *db.TenantManager) *Dispatcher {
	if tenants == nil {
		return nil
	}
	return &Dispatcher{
		tenants: tenants,
		client:  &http.Client{Timeout: 20 * time.Second},
	}
}

// Emit runs delivery asynchronously (does not block the request path).
func (d *Dispatcher) Emit(ctx context.Context, eventType, tableID string, data map[string]any) {
	if d == nil {
		return
	}
	if _, ok := allRecordHooks[eventType]; !ok {
		return
	}
	payload := map[string]any{
		"type":    eventType,
		"tableId": tableID,
		"data":    data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	asyncCtx := tenant.WithTenantID(context.Background(), tenant.FromContext(ctx))
	go d.deliver(asyncCtx, eventType, tableID, body)
}

func (d *Dispatcher) deliver(ctx context.Context, eventType, tableID string, body []byte) {
	tid := strings.TrimSpace(tenant.FromContext(ctx))
	if tid == "" || d.tenants == nil {
		return
	}
	pool := d.tenants.MetaPool()
	rows, err := pool.Query(ctx, `
		SELECT sink_type, target_url, COALESCE(table_filter, ''), events, headers, secret, sink_config
		FROM lc_webhooks
		WHERE enabled = true AND tenant_id = $1
	`, tid)
	if err != nil {
		log.Printf("sink: list: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var sinkType, targetURL, tableFilter, secret string
		var eventsRaw, headersRaw, sinkConfigRaw []byte
		if err := rows.Scan(&sinkType, &targetURL, &tableFilter, &eventsRaw, &headersRaw, &secret, &sinkConfigRaw); err != nil {
			continue
		}
		if tableFilter != "" && tableFilter != tableID {
			continue
		}
		if !eventMatches(parseEvents(eventsRaw), eventType) {
			continue
		}
		sinkType = NormalizeSinkType(sinkType)
		sinkConfig := parseSinkConfig(sinkConfigRaw)
		d.deliverOne(ctx, sinkType, targetURL, parseHeaders(headersRaw), secret, sinkConfig, body)
	}
}

func (d *Dispatcher) deliverOne(ctx context.Context, sinkType, targetURL string, headers map[string]string, secret string, sinkConfig map[string]any, body []byte) {
	switch sinkType {
	case TypeWebhook:
		url := strings.TrimSpace(targetURL)
		if url == "" {
			url = configString(sinkConfig, "url")
		}
		if secret == "" {
			secret = configString(sinkConfig, "secret")
		}
		d.deliverWebhook(ctx, url, headers, secret, body)
	case TypeRedis:
		deliverRedis(ctx, sinkConfig, body)
	case TypeRabbitMQ:
		deliverRabbitMQ(ctx, sinkConfig, body)
	case TypeKafka:
		deliverKafka(ctx, sinkConfig, body)
	default:
		log.Printf("sink: unknown type %q", sinkType)
	}
}
