package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
)

func (s *LowcodeService) CreateWebhook(ctx context.Context, req *apiv1.CreateWebhookRequest) (*apiv1.CreateWebhookResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	name := req.Name
	if name == "" || req.TargetUrl == "" {
		return nil, fmt.Errorf("name and target_url are required")
	}
	eventsJSON, _ := json.Marshal(req.Events)
	headersMap := req.Headers
	hJSON, _ := json.Marshal(headersMap)
	const q = `
		INSERT INTO lc_webhooks (tenant_id, name, target_url, table_filter, events, headers, enabled, secret)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8)
		RETURNING id, name, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
	`
	row := meta.QueryRow(ctx, q,
		tid,
		name,
		req.TargetUrl,
		req.TableFilter,
		string(eventsJSON),
		string(hJSON),
		req.Enabled,
		req.Secret,
	)
	w, err := scanWebhook(row)
	if err != nil {
		return nil, err
	}
	return &apiv1.CreateWebhookResponse{Webhook: w}, nil
}

func (s *LowcodeService) ListWebhooks(ctx context.Context, _ *apiv1.ListWebhooksRequest) (*apiv1.ListWebhooksResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	rows, err := meta.Query(ctx, `
		SELECT id, name, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
		FROM lc_webhooks WHERE tenant_id = $1 ORDER BY created_at
	`, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out apiv1.ListWebhooksResponse
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		out.Webhooks = append(out.Webhooks, w)
	}
	return &out, rows.Err()
}

func (s *LowcodeService) DeleteWebhook(ctx context.Context, req *apiv1.DeleteWebhookRequest) (*apiv1.DeleteWebhookResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	if req.Id == "" {
		return nil, fmt.Errorf("id is required")
	}
	_, err = meta.Exec(ctx, `DELETE FROM lc_webhooks WHERE id = $1 AND tenant_id = $2`, req.Id, tid)
	if err != nil {
		return nil, err
	}
	return &apiv1.DeleteWebhookResponse{}, nil
}

func (s *LowcodeService) UpdateWebhook(ctx context.Context, req *apiv1.UpdateWebhookRequest) (*apiv1.UpdateWebhookResponse, error) {
	tid, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.tenants.MetaPool()
	if req.Id == "" {
		return nil, fmt.Errorf("id is required")
	}
	eventsJSON, _ := json.Marshal(req.Events)
	hJSON, _ := json.Marshal(req.Headers)

	if req.Secret != "" {
		const q = `
			UPDATE lc_webhooks SET
				name = $2, target_url = $3, table_filter = $4, events = $5::jsonb,
				headers = $6::jsonb, enabled = $7, secret = $8, updated_at = now()
			WHERE id = $1 AND tenant_id = $9
			RETURNING id, name, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
		`
		row := meta.QueryRow(ctx, q, req.Id, req.Name, req.TargetUrl, req.TableFilter,
			string(eventsJSON), string(hJSON), req.Enabled, req.Secret, tid)
		w, err := scanWebhook(row)
		if err != nil {
			return nil, err
		}
		return &apiv1.UpdateWebhookResponse{Webhook: w}, nil
	}
	const q = `
		UPDATE lc_webhooks SET
			name = $2, target_url = $3, table_filter = $4, events = $5::jsonb,
			headers = $6::jsonb, enabled = $7, updated_at = now()
		WHERE id = $1 AND tenant_id = $8
		RETURNING id, name, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
	`
	row := meta.QueryRow(ctx, q, req.Id, req.Name, req.TargetUrl, req.TableFilter,
		string(eventsJSON), string(hJSON), req.Enabled, tid)
	w, err := scanWebhook(row)
	if err != nil {
		return nil, err
	}
	return &apiv1.UpdateWebhookResponse{Webhook: w}, nil
}

type webhookScanner interface {
	Scan(dest ...any) error
}

func scanWebhook(row webhookScanner) (*apiv1.Webhook, error) {
	var w apiv1.Webhook
	var eventsRaw, headersRaw []byte
	var secret string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&w.Id, &w.Name, &w.TargetUrl, &w.TableFilter, &eventsRaw, &headersRaw, &w.Enabled, &secret, &createdAt, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("webhook not found")
		}
		return nil, err
	}
	var ev []string
	if len(eventsRaw) > 0 {
		_ = json.Unmarshal(eventsRaw, &ev)
	}
	w.Events = ev
	var hm map[string]any
	if len(headersRaw) > 0 {
		_ = json.Unmarshal(headersRaw, &hm)
	}
	if hm != nil {
		w.Headers = hm
	}
	w.HasSecret = secret != ""
	w.CreatedAt = createdAt
	w.UpdatedAt = updatedAt
	return &w, nil
}
