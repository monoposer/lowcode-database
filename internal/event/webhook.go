package event

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Webhook is a tenant-scoped HTTP subscription.
type Webhook struct {
	ID          string
	TenantID    string
	URL         string
	Secret      string
	EventPrefix string
	Enabled     bool
}

// WebhookStore loads subscriptions from meta.
type WebhookStore interface {
	ListEnabledWebhooks(ctx context.Context, tenantID string) ([]Webhook, error)
}

// DeliverWebhooks POSTs matching subscriptions. Used as a Bus Handler.
func DeliverWebhooks(store WebhookStore, client *http.Client) Handler {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return func(ctx context.Context, e Envelope) error {
		if store == nil || e.TenantID == "" {
			return nil
		}
		hooks, err := store.ListEnabledWebhooks(ctx, e.TenantID)
		if err != nil {
			return err
		}
		body, err := json.Marshal(e)
		if err != nil {
			return err
		}
		var last error
		for _, h := range hooks {
			if !webhookMatches(h.EventPrefix, e.Type) {
				continue
			}
			if err := postWebhook(ctx, client, h, e.Type, body); err != nil {
				last = err
			}
		}
		return last
	}
}

func webhookMatches(prefix, eventType string) bool {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(eventType, prefix)
}

func postWebhook(ctx context.Context, client *http.Client, h Webhook, eventType string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Event-Type", eventType)
	if h.Secret != "" {
		mac := hmac.New(sha256.New, []byte(h.Secret))
		_, _ = mac.Write(body)
		req.Header.Set("X-Webhook-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook %s: HTTP %d", h.URL, resp.StatusCode)
	}
	return nil
}
