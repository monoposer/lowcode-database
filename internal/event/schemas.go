package event

import (
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/solat/lowcode-database/pkg/eventschema"
	"sync"
)

// Re-export event type constants from pkg/eventschema.
const (
	RecordsAfterInsert     = eventschema.RecordsAfterInsert
	RecordsAfterUpdate     = eventschema.RecordsAfterUpdate
	RecordsAfterDelete     = eventschema.RecordsAfterDelete
	RecordsAfterBulkUpsert = eventschema.RecordsAfterBulkUpsert
	RecordsAfterBulkDelete = eventschema.RecordsAfterBulkDelete
	RecordsAfterBulkImport = eventschema.RecordsAfterBulkImport

	MetadataTableCreated      = eventschema.MetadataTableCreated
	MetadataTableDeleted      = eventschema.MetadataTableDeleted
	MetadataTableRenamed      = eventschema.MetadataTableRenamed
	MetadataColumnCreated     = eventschema.MetadataColumnCreated
	MetadataColumnUpdated     = eventschema.MetadataColumnUpdated
	MetadataColumnDeleted     = eventschema.MetadataColumnDeleted
	MetadataChoiceCreated     = eventschema.MetadataChoiceCreated
	MetadataChoiceUpdated     = eventschema.MetadataChoiceUpdated
	MetadataChoiceDeleted     = eventschema.MetadataChoiceDeleted
	MetadataRelationCreated   = eventschema.MetadataRelationCreated
	MetadataRelationDeleted   = eventschema.MetadataRelationDeleted
	MetadataIndexCreated      = eventschema.MetadataIndexCreated
	MetadataIndexDeleted      = eventschema.MetadataIndexDeleted
	MetadataDataSourceCreated = eventschema.MetadataDataSourceCreated
	MetadataDataSourceUpdated = eventschema.MetadataDataSourceUpdated
	MetadataDataSourceDeleted = eventschema.MetadataDataSourceDeleted
)

// Schemas maps event type -> JSON Schema for envelope.data (from pkg/eventschema).
var Schemas = eventschema.PayloadSchemas()

// EnvelopeSchema returns the JSON Schema for the delivery envelope.
func EnvelopeSchema() json.RawMessage { return eventschema.EnvelopeSchema() }

// ListSchemaTypes returns sorted event type names that have payload schemas.
func ListSchemaTypes() []string { return eventschema.PayloadSchemaTypes() }

func ValidEventType(t string) bool { return eventschema.ValidType(t) }

var (
	promEventDeliveryTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "lowcode_event_delivery_total",
		Help: "Event webhook delivery attempts by outcome",
	}, []string{"tenant_id", "status"})

	promEventDeliveryRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "lowcode_event_delivery_retries_total",
		Help: "Event webhook delivery retries (attempt > 1)",
	}, []string{"tenant_id"})

	promEventDeadLetterTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "lowcode_event_dead_letter_total",
		Help: "Event deliveries exhausted retries and were dead-lettered",
	}, []string{"tenant_id"})

	promEventRegisterOnce sync.Once
)

func registerEventPrometheus() {
	promEventRegisterOnce.Do(func() {
		prometheus.MustRegister(promEventDeliveryTotal, promEventDeliveryRetries, promEventDeadLetterTotal)
	})
}

type deliveryMetrics struct {
	enabled bool
}

func newDeliveryMetrics(enabled bool) *deliveryMetrics {
	if enabled {
		registerEventPrometheus()
	}
	return &deliveryMetrics{enabled: enabled}
}

func (m *deliveryMetrics) record(tenantID, status string, attempts int) {
	if m == nil || !m.enabled {
		return
	}
	if tenantID == "" {
		tenantID = "unknown"
	}
	promEventDeliveryTotal.With(prometheus.Labels{"tenant_id": tenantID, "status": status}).Inc()
	if attempts > 1 {
		promEventDeliveryRetries.With(prometheus.Labels{"tenant_id": tenantID}).Add(float64(attempts - 1))
	}
	if status == DeliveryStatusDeadLetter {
		promEventDeadLetterTotal.With(prometheus.Labels{"tenant_id": tenantID}).Inc()
	}
}
