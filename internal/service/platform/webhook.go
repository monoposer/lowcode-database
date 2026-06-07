package platform

import (
	"context"
	"fmt"
	"strings"

	"github.com/monoposer/lowcode-database/internal/event"
)

type CreateWebhookRequest struct {
	URL         string `json:"url"`
	Secret      string `json:"secret,omitempty"`
	EventPrefix string `json:"eventPrefix,omitempty"`
}

type WebhookDTO struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	EventPrefix string `json:"eventPrefix,omitempty"`
	Enabled     bool   `json:"enabled"`
	HasSecret   bool   `json:"hasSecret"`
}

type ListWebhooksResponse struct {
	Webhooks []WebhookDTO `json:"webhooks"`
}

func (s *Platform) CreateWebhook(ctx context.Context, req *CreateWebhookRequest) (*WebhookDTO, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("url is required")
	}
	w, err := s.B.Tenants.InsertWebhook(ctx, tid, req.URL, req.Secret, req.EventPrefix)
	if err != nil {
		return nil, err
	}
	return webhookDTO(w), nil
}

func (s *Platform) ListWebhooks(ctx context.Context) (*ListWebhooksResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	list, err := s.B.Tenants.ListWebhooks(ctx, tid)
	if err != nil {
		return nil, err
	}
	out := make([]WebhookDTO, 0, len(list))
	for _, w := range list {
		out = append(out, *webhookDTO(w))
	}
	return &ListWebhooksResponse{Webhooks: out}, nil
}

func (s *Platform) DeleteWebhook(ctx context.Context, id string) error {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return err
	}
	return s.B.Tenants.DeleteWebhook(ctx, tid, id)
}

func webhookDTO(w event.Webhook) *WebhookDTO {
	return &WebhookDTO{
		ID:          w.ID,
		URL:         w.URL,
		EventPrefix: w.EventPrefix,
		Enabled:     w.Enabled,
		HasSecret:   w.Secret != "",
	}
}
