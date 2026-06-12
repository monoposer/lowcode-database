package eventschema

import "encoding/json"

// Envelope is the JSON document pushed to every event sink.
type Envelope struct {
	Type       string          `json:"type"`
	TenantID   string          `json:"tenantId"`
	TableID    string          `json:"tableId,omitempty"`
	OccurredAt string          `json:"occurredAt"`
	Data       json.RawMessage `json:"data"`
}
