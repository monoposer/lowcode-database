package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/monoposer/lowcode-database/internal/event"
)

// ListEnabledWebhooks implements event.WebhookStore.
func (m *TenantManager) ListEnabledWebhooks(ctx context.Context, tenantID string) ([]event.Webhook, error) {
	if m == nil || m.metaPool == nil {
		return nil, nil
	}
	rows, err := m.metaPool.Query(ctx, `
		SELECT id::text, tenant_id, url, secret, event_prefix, enabled
		FROM lc_event_webhooks
		WHERE tenant_id = $1 AND enabled = TRUE
		ORDER BY created_at
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []event.Webhook
	for rows.Next() {
		var w event.Webhook
		if err := rows.Scan(&w.ID, &w.TenantID, &w.URL, &w.Secret, &w.EventPrefix, &w.Enabled); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (m *TenantManager) InsertWebhook(ctx context.Context, tenantID, url, secret, prefix string) (event.Webhook, error) {
	tenantID = strings.TrimSpace(tenantID)
	url = strings.TrimSpace(url)
	if tenantID == "" || url == "" {
		return event.Webhook{}, fmt.Errorf("tenant_id and url are required")
	}
	id := uuid.NewString()
	_, err := m.metaPool.Exec(ctx, `
		INSERT INTO lc_event_webhooks (id, tenant_id, url, secret, event_prefix, enabled)
		VALUES ($1::uuid, $2, $3, $4, $5, TRUE)
	`, id, tenantID, url, secret, strings.TrimSpace(prefix))
	if err != nil {
		return event.Webhook{}, err
	}
	return event.Webhook{ID: id, TenantID: tenantID, URL: url, Secret: secret, EventPrefix: prefix, Enabled: true}, nil
}

func (m *TenantManager) DeleteWebhook(ctx context.Context, tenantID, id string) error {
	tag, err := m.metaPool.Exec(ctx, `
		DELETE FROM lc_event_webhooks WHERE tenant_id = $1 AND id = $2::uuid
	`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("webhook not found")
	}
	return nil
}

func (m *TenantManager) ListWebhooks(ctx context.Context, tenantID string) ([]event.Webhook, error) {
	rows, err := m.metaPool.Query(ctx, `
		SELECT id::text, tenant_id, url, secret, event_prefix, enabled
		FROM lc_event_webhooks WHERE tenant_id = $1 ORDER BY created_at
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []event.Webhook
	for rows.Next() {
		var w event.Webhook
		if err := rows.Scan(&w.ID, &w.TenantID, &w.URL, &w.Secret, &w.EventPrefix, &w.Enabled); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
