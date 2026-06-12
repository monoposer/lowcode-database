package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/solat/lowcode-database/internal/apiv1/platform"
	"github.com/solat/lowcode-database/internal/event"
	"time"
)

type eventSinkScanner interface {
	Scan(dest ...any) error
}

func scanEventSink(row eventSinkScanner) (*platform.EventSink, error) {
	var es platform.EventSink
	var eventsRaw, headersRaw, sinkConfigRaw []byte
	var secret string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&es.Id, &es.Name, &es.Sink, &sinkConfigRaw, &es.TargetUrl, &es.TableFilter,
		&eventsRaw, &headersRaw, &es.Enabled, &secret, &createdAt, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("event sink not found")
		}
		return nil, err
	}
	es.Sink = event.NormalizeSink(es.Sink)
	_ = json.Unmarshal(eventsRaw, &es.EventTypes)
	var hm map[string]any
	_ = json.Unmarshal(headersRaw, &hm)
	es.Headers = hm
	var sc map[string]any
	_ = json.Unmarshal(sinkConfigRaw, &sc)
	es.SinkConfig = sc
	es.HasSecret = secret != ""
	es.CreatedAt = createdAt
	es.UpdatedAt = updatedAt
	return &es, nil
}

func (s *Platform) CreateEventSink(ctx context.Context, req *platform.CreateEventSinkRequest) (*platform.CreateEventSinkResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	sinkName := event.SinkWebhook
	deliveryURL, err := event.ResolveDeliveryURL(sinkName, req.TargetUrl, req.SinkConfig)
	if err != nil {
		return nil, err
	}
	if err := event.ValidateDeliveryURL(deliveryURL); err != nil {
		return nil, err
	}
	eventsJSON, _ := json.Marshal(req.EventTypes)
	hJSON, _ := json.Marshal(req.Headers)
	sinkConfigJSON, _ := json.Marshal(req.SinkConfig)
	meta := s.B.Tenants.MetaPool()
	row := meta.QueryRow(ctx, `
		INSERT INTO lc_event_sinks (tenant_id, name, sink, sink_config, target_url, table_filter, events, headers, enabled, secret)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7::jsonb, $8::jsonb, $9, $10)
		RETURNING id, name, sink, sink_config, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
	`, tid, req.Name, sinkName, string(sinkConfigJSON), deliveryURL, req.TableFilter,
		string(eventsJSON), string(hJSON), req.Enabled, req.Secret)
	es, err := scanEventSink(row)
	if err != nil {
		return nil, err
	}
	return &platform.CreateEventSinkResponse{EventSink: es}, nil
}

func (s *Platform) ListEventSinks(ctx context.Context, _ *platform.ListEventSinksRequest) (*platform.ListEventSinksResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	rows, err := meta.Query(ctx, `
		SELECT id, name, sink, sink_config, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
		FROM lc_event_sinks WHERE tenant_id = $1 ORDER BY created_at
	`, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out platform.ListEventSinksResponse
	for rows.Next() {
		es, err := scanEventSink(rows)
		if err != nil {
			return nil, err
		}
		out.EventSinks = append(out.EventSinks, es)
	}
	return &out, rows.Err()
}

func (s *Platform) DeleteEventSink(ctx context.Context, req *platform.DeleteEventSinkRequest) (*platform.DeleteEventSinkResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, fmt.Errorf("id is required")
	}
	_, err = s.B.Tenants.MetaPool().Exec(ctx, `DELETE FROM lc_event_sinks WHERE id = $1 AND tenant_id = $2`, req.Id, tid)
	if err != nil {
		return nil, err
	}
	return &platform.DeleteEventSinkResponse{}, nil
}

func (s *Platform) UpdateEventSink(ctx context.Context, req *platform.UpdateEventSinkRequest) (*platform.UpdateEventSinkResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, fmt.Errorf("id is required")
	}
	meta := s.B.Tenants.MetaPool()
	existing, err := scanEventSink(meta.QueryRow(ctx, `
		SELECT id, name, sink, sink_config, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
		FROM lc_event_sinks WHERE id = $1 AND tenant_id = $2
	`, req.Id, tid))
	if err != nil {
		return nil, err
	}
	targetURL := existing.TargetUrl
	if req.TargetUrl != "" {
		targetURL = req.TargetUrl
	}
	sinkConfig := existing.SinkConfig
	if req.SinkConfig != nil {
		sinkConfig = req.SinkConfig
	}
	deliveryURL, err := event.ResolveDeliveryURL(event.SinkWebhook, targetURL, sinkConfig)
	if err != nil {
		return nil, err
	}
	if err := event.ValidateDeliveryURL(deliveryURL); err != nil {
		return nil, err
	}
	sinkName := event.SinkWebhook
	targetURL = deliveryURL
	name := existing.Name
	if req.Name != "" {
		name = req.Name
	}
	tableFilter := existing.TableFilter
	if req.TableFilter != "" || req.Name != "" || req.TargetUrl != "" || req.Sink != "" || req.SinkConfig != nil {
		tableFilter = req.TableFilter
	}
	eventTypes := existing.EventTypes
	if req.EventTypes != nil {
		eventTypes = req.EventTypes
	}
	headers := existing.Headers
	if req.Headers != nil {
		headers = req.Headers
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	eventsJSON, _ := json.Marshal(eventTypes)
	hJSON, _ := json.Marshal(headers)
	sinkConfigJSON, _ := json.Marshal(sinkConfig)

	if req.Secret != "" {
		row := meta.QueryRow(ctx, `
			UPDATE lc_event_sinks SET
				name = $2, sink = $3, sink_config = $4::jsonb, target_url = $5, table_filter = $6,
				events = $7::jsonb, headers = $8::jsonb, enabled = $9, secret = $10, updated_at = now()
			WHERE id = $1 AND tenant_id = $11
			RETURNING id, name, sink, sink_config, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
		`, req.Id, name, sinkName, string(sinkConfigJSON), targetURL, tableFilter,
			string(eventsJSON), string(hJSON), enabled, req.Secret, tid)
		es, err := scanEventSink(row)
		if err != nil {
			return nil, err
		}
		return &platform.UpdateEventSinkResponse{EventSink: es}, nil
	}
	row := meta.QueryRow(ctx, `
		UPDATE lc_event_sinks SET
			name = $2, sink = $3, sink_config = $4::jsonb, target_url = $5, table_filter = $6,
			events = $7::jsonb, headers = $8::jsonb, enabled = $9, updated_at = now()
		WHERE id = $1 AND tenant_id = $10
		RETURNING id, name, sink, sink_config, target_url, table_filter, events, headers, enabled, secret, created_at, updated_at
	`, req.Id, name, sinkName, string(sinkConfigJSON), targetURL, tableFilter,
		string(eventsJSON), string(hJSON), enabled, tid)
	es, err := scanEventSink(row)
	if err != nil {
		return nil, err
	}
	return &platform.UpdateEventSinkResponse{EventSink: es}, nil
}
