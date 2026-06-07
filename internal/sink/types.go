package sink

// NocoDB-style event type strings (subset; extend as needed).
const (
	RecordsAfterInsert     = "records.after.insert"
	RecordsAfterUpdate     = "records.after.update"
	RecordsAfterDelete     = "records.after.delete"
	RecordsAfterBulkUpsert = "records.after.bulkUpsert"
	RecordsAfterBulkDelete = "records.after.bulkDelete"
	RecordsAfterBulkImport = "records.after.bulkImport"
)

// Sink delivery backends (Sequin-style).
const (
	TypeWebhook  = "webhook"
	TypeRedis    = "redis"
	TypeRabbitMQ = "rabbitmq"
	TypeKafka    = "kafka"
)

var allRecordHooks = map[string]struct{}{
	RecordsAfterInsert:     {},
	RecordsAfterUpdate:     {},
	RecordsAfterDelete:     {},
	RecordsAfterBulkUpsert: {},
	RecordsAfterBulkDelete: {},
	RecordsAfterBulkImport: {},
}

var knownSinkTypes = map[string]struct{}{
	TypeWebhook:  {},
	TypeRedis:    {},
	TypeRabbitMQ: {},
	TypeKafka:    {},
}

func NormalizeSinkType(t string) string {
	if t == "" {
		return TypeWebhook
	}
	return t
}

func ValidSinkType(t string) bool {
	_, ok := knownSinkTypes[t]
	return ok
}
