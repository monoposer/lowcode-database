package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/solat/lowcode-database/internal/db"
	"github.com/solat/lowcode-database/internal/tenant"
)

// NocoDB-style event type strings (subset; extend as needed).
const (
	RecordsAfterInsert     = "records.after.insert"
	RecordsAfterUpdate     = "records.after.update"
	RecordsAfterDelete     = "records.after.delete"
	RecordsAfterBulkUpsert = "records.after.bulkUpsert"
	RecordsAfterBulkDelete = "records.after.bulkDelete"
	RecordsAfterBulkImport = "records.after.bulkImport"
)

var allRecordHooks = map[string]struct{}{
	RecordsAfterInsert:     {},
	RecordsAfterUpdate:     {},
	RecordsAfterDelete:     {},
	RecordsAfterBulkUpsert: {},
	RecordsAfterBulkDelete: {},
	RecordsAfterBulkImport: {},
}

// Dispatcher loads lc_webhooks and POSTs JSON payloads (NocoDB-like: top-level "type", "tableId", "data").
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
		SELECT id, target_url, COALESCE(table_filter, ''), events, headers, secret
		FROM lc_webhooks
		WHERE enabled = true AND tenant_id = $1
	`, tid)
	if err != nil {
		log.Printf("webhook: list: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, targetURL, tableFilter, secret string
		var eventsRaw, headersRaw []byte
		if err := rows.Scan(&id, &targetURL, &tableFilter, &eventsRaw, &headersRaw, &secret); err != nil {
			continue
		}
		if tableFilter != "" && tableFilter != tableID {
			continue
		}
		var evList []string
		if len(eventsRaw) > 0 && string(eventsRaw) != "null" {
			_ = json.Unmarshal(eventsRaw, &evList)
		}
		if len(evList) > 0 {
			found := false
			for _, e := range evList {
				if e == eventType {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		headers := map[string]string{}
		if len(headersRaw) > 0 && string(headersRaw) != "null" {
			_ = json.Unmarshal(headersRaw, &headers)
		}
		d.postOne(ctx, targetURL, headers, secret, body)
	}
}

func (d *Dispatcher) postOne(ctx context.Context, urlStr string, headers map[string]string, secret string, body []byte) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		req.Header.Set("X-Lowcode-Signature", hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := d.client.Do(req)
	if err != nil {
		log.Printf("webhook POST %s: %v", urlStr, err)
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("webhook POST %s: status %s", urlStr, resp.Status)
	}
}
