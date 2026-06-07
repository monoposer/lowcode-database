package event

import (
	"context"
	"time"
)

// Envelope is a domain event published on EventBus.
type Envelope struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	TenantID string         `json:"tenantId"`
	TableID  string         `json:"tableId,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Time     time.Time      `json:"time"`
}

// Handler consumes one envelope. Return error to retry (Redis Stream ACK skipped).
type Handler func(ctx context.Context, e Envelope) error

// Bus is the process-wide event transport.
type Bus interface {
	Publish(ctx context.Context, e Envelope) error
	// Subscribe runs until ctx is cancelled. group/consumer are used by Redis Streams.
	Subscribe(ctx context.Context, group, consumer string, h Handler) error
	Close() error
}

// BusType maps internal constants onto the bus namespace (metadata.* → schema.*).
func BusType(eventType string) string {
	const meta = "metadata."
	if len(eventType) > len(meta) && eventType[:len(meta)] == meta {
		return "schema." + eventType[len(meta):]
	}
	return eventType
}
