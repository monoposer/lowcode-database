package platform

import (
	"time"
)

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
