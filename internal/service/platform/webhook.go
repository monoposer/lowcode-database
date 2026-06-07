package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/sink"
)

func (s *Platform) CreateWebhook(ctx context.Context, req *apiv1.CreateWebhookRequest) (*apiv1.CreateWebhookResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	name := req.Name
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	sinkType := sink.NormalizeSinkType(req.SinkType)
	if !sink.ValidSinkType(sinkType) {
		return nil, fmt.Errorf("invalid sink_type %q", req.SinkType)
	}
	if err := sink.ValidateCreate(sinkType, req.TargetUrl, req.SinkConfig); err != nil {
		return nil, err
	}
	eventsJSON, _ := json.Marshal(req.Events)
	headersMap := req.Headers
	hJSON, _ := json.Marshal(headersMap)
	sinkConfigJSON, _ := json.Marshal(req.SinkConfig)
	const q = `
		INSERT INTO lc_webhooks (tenant_id, name, sink_type, sink_config, target_url, table_filter, events, headers, enabled, secret)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7::jsonb, $8::jsonb, $9, $10)
		RETURNING id, name, sink_type, sink_config, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
	`
	row := meta.QueryRow(ctx, q,
		tid,
		name,
		sinkType,
		string(sinkConfigJSON),
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

func (s *Platform) ListWebhooks(ctx context.Context, _ *apiv1.ListWebhooksRequest) (*apiv1.ListWebhooksResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	rows, err := meta.Query(ctx, `
		SELECT id, name, sink_type, sink_config, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
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

func (s *Platform) DeleteWebhook(ctx context.Context, req *apiv1.DeleteWebhookRequest) (*apiv1.DeleteWebhookResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	if req.Id == "" {
		return nil, fmt.Errorf("id is required")
	}
	_, err = meta.Exec(ctx, `DELETE FROM lc_webhooks WHERE id = $1 AND tenant_id = $2`, req.Id, tid)
	if err != nil {
		return nil, err
	}
	return &apiv1.DeleteWebhookResponse{}, nil
}

func (s *Platform) UpdateWebhook(ctx context.Context, req *apiv1.UpdateWebhookRequest) (*apiv1.UpdateWebhookResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	if req.Id == "" {
		return nil, fmt.Errorf("id is required")
	}
	existing, err := scanWebhook(meta.QueryRow(ctx, `
		SELECT id, name, sink_type, sink_config, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
		FROM lc_webhooks WHERE id = $1 AND tenant_id = $2
	`, req.Id, tid))
	if err != nil {
		return nil, err
	}
	sinkType := existing.SinkType
	if req.SinkType != "" {
		sinkType = sink.NormalizeSinkType(req.SinkType)
		if !sink.ValidSinkType(sinkType) {
			return nil, fmt.Errorf("invalid sink_type %q", req.SinkType)
		}
	}
	targetURL := existing.TargetUrl
	if req.TargetUrl != "" {
		targetURL = req.TargetUrl
	}
	sinkConfig := existing.SinkConfig
	if req.SinkConfig != nil {
		sinkConfig = req.SinkConfig
	}
	if err := sink.ValidateCreate(sinkType, targetURL, sinkConfig); err != nil {
		return nil, err
	}
	name := existing.Name
	if req.Name != "" {
		name = req.Name
	}
	tableFilter := existing.TableFilter
	if req.TableFilter != "" || (req.Name != "" || req.TargetUrl != "" || req.SinkType != "" || req.SinkConfig != nil) {
		tableFilter = req.TableFilter
	}
	events := existing.Events
	if req.Events != nil {
		events = req.Events
	}
	headers := existing.Headers
	if req.Headers != nil {
		headers = req.Headers
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	eventsJSON, _ := json.Marshal(events)
	hJSON, _ := json.Marshal(headers)
	sinkConfigJSON, _ := json.Marshal(sinkConfig)

	if req.Secret != "" {
		const q = `
			UPDATE lc_webhooks SET
				name = $2, sink_type = $3, sink_config = $4::jsonb, target_url = $5, table_filter = $6,
				events = $7::jsonb, headers = $8::jsonb, enabled = $9, secret = $10, updated_at = now()
			WHERE id = $1 AND tenant_id = $11
			RETURNING id, name, sink_type, sink_config, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
		`
		row := meta.QueryRow(ctx, q, req.Id, name, sinkType, string(sinkConfigJSON), targetURL, tableFilter,
			string(eventsJSON), string(hJSON), enabled, req.Secret, tid)
		w, err := scanWebhook(row)
		if err != nil {
			return nil, err
		}
		return &apiv1.UpdateWebhookResponse{Webhook: w}, nil
	}
	const q = `
		UPDATE lc_webhooks SET
			name = $2, sink_type = $3, sink_config = $4::jsonb, target_url = $5, table_filter = $6,
			events = $7::jsonb, headers = $8::jsonb, enabled = $9, updated_at = now()
		WHERE id = $1 AND tenant_id = $10
		RETURNING id, name, sink_type, sink_config, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
	`
	row := meta.QueryRow(ctx, q, req.Id, name, sinkType, string(sinkConfigJSON), targetURL, tableFilter,
		string(eventsJSON), string(hJSON), enabled, tid)
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
	var eventsRaw, headersRaw, sinkConfigRaw []byte
	var secret string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&w.Id, &w.Name, &w.SinkType, &sinkConfigRaw, &w.TargetUrl, &w.TableFilter, &eventsRaw, &headersRaw, &w.Enabled, &secret, &createdAt, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("webhook not found")
		}
		return nil, err
	}
	w.SinkType = sink.NormalizeSinkType(w.SinkType)
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
	var sc map[string]any
	if len(sinkConfigRaw) > 0 {
		_ = json.Unmarshal(sinkConfigRaw, &sc)
	}
	if sc != nil {
		w.SinkConfig = sc
	}
	w.HasSecret = secret != ""
	w.CreatedAt = createdAt
	w.UpdatedAt = updatedAt
	return &w, nil
}
