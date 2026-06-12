package platform

import (
	"encoding/json"
	"time"
)

type EventSink struct {
	Id          string         `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	Sink        string         `json:"sink,omitempty"`
	SinkConfig  map[string]any `json:"sinkConfig,omitempty"`
	TargetUrl   string         `json:"targetUrl,omitempty"`
	TableFilter string         `json:"tableFilter,omitempty"`
	EventTypes  []string       `json:"eventTypes,omitempty"`
	Headers     map[string]any `json:"headers,omitempty"`
	Enabled     bool           `json:"enabled,omitempty"`
	HasSecret   bool           `json:"hasSecret,omitempty"`
	CreatedAt   time.Time      `json:"createdAt,omitempty"`
	UpdatedAt   time.Time      `json:"updatedAt,omitempty"`
}

type CreateEventSinkRequest struct {
	Name        string         `json:"name,omitempty"`
	Sink        string         `json:"sink,omitempty"`
	SinkConfig  map[string]any `json:"sinkConfig,omitempty"`
	TargetUrl   string         `json:"targetUrl,omitempty"`
	TableFilter string         `json:"tableFilter,omitempty"`
	EventTypes  []string       `json:"eventTypes,omitempty"`
	Headers     map[string]any `json:"headers,omitempty"`
	Enabled     bool           `json:"enabled,omitempty"`
	Secret      string         `json:"secret,omitempty"`
}

type CreateEventSinkResponse struct {
	EventSink *EventSink `json:"eventSink,omitempty"`
}

type ListEventSinksRequest struct{}

type ListEventSinksResponse struct {
	EventSinks []*EventSink `json:"eventSinks,omitempty"`
}

type DeleteEventSinkRequest struct {
	Id string `json:"id,omitempty"`
}

type DeleteEventSinkResponse struct{}

type UpdateEventSinkRequest struct {
	Id          string         `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	Sink        string         `json:"sink,omitempty"`
	SinkConfig  map[string]any `json:"sinkConfig,omitempty"`
	TargetUrl   string         `json:"targetUrl,omitempty"`
	TableFilter string         `json:"tableFilter,omitempty"`
	EventTypes  []string       `json:"eventTypes,omitempty"`
	Headers     map[string]any `json:"headers,omitempty"`
	Enabled     *bool          `json:"enabled,omitempty"`
	Secret      string         `json:"secret,omitempty"`
}

type UpdateEventSinkResponse struct {
	EventSink *EventSink `json:"eventSink,omitempty"`
}

type ListEventSchemasRequest struct{}

type ListEventSchemasResponse struct {
	EnvelopeSchema json.RawMessage            `json:"envelopeSchema,omitempty"`
	Schemas        map[string]json.RawMessage `json:"schemas,omitempty"`
}

type EventDeliveryLogEntry struct {
	Id        string         `json:"id,omitempty"`
	SinkId    string         `json:"sinkId,omitempty"`
	EventType string         `json:"eventType,omitempty"`
	TableId   string         `json:"tableId,omitempty"`
	TargetUrl string         `json:"targetUrl,omitempty"`
	Attempts  int            `json:"attempts,omitempty"`
	Status    string         `json:"status,omitempty"`
	LastError string         `json:"lastError,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt time.Time      `json:"createdAt,omitempty"`
}

type ListEventDeliveryLogRequest struct {
	Limit  int    `json:"limit,omitempty"`
	Status string `json:"status,omitempty"`
}

type ListEventDeliveryLogResponse struct {
	Entries []*EventDeliveryLogEntry `json:"entries,omitempty"`
}

type SchemaAuditEntry struct {
	Id           string         `json:"id,omitempty"`
	Action       string         `json:"action,omitempty"`
	ResourceType string         `json:"resourceType,omitempty"`
	ResourceId   string         `json:"resourceId,omitempty"`
	TableId      string         `json:"tableId,omitempty"`
	Detail       map[string]any `json:"detail,omitempty"`
	OccurredAt   time.Time      `json:"occurredAt,omitempty"`
}

type ListSchemaAuditRequest struct {
	Limit   int    `json:"limit,omitempty"`
	TableId string `json:"tableId,omitempty"`
}

type ListSchemaAuditResponse struct {
	Entries []*SchemaAuditEntry `json:"entries,omitempty"`
}
