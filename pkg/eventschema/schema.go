package eventschema

import (
	"embed"
	"encoding/json"
	"sort"
	"strings"
)

//go:embed schema/envelope.json schema/records/*.json schema/metadata/*.json
var schemaFS embed.FS

const schemaDir = "schema"

var payloadTypes = []string{
	RecordsAfterInsert, RecordsAfterUpdate, RecordsAfterDelete,
	RecordsAfterBulkUpsert, RecordsAfterBulkDelete, RecordsAfterBulkImport,
	MetadataTableCreated, MetadataTableDeleted, MetadataTableRenamed,
	MetadataColumnCreated, MetadataColumnUpdated, MetadataColumnDeleted,
	MetadataChoiceCreated, MetadataChoiceUpdated, MetadataChoiceDeleted,
	MetadataRelationCreated, MetadataRelationDeleted,
	MetadataIndexCreated, MetadataIndexDeleted,
	MetadataDataSourceCreated, MetadataDataSourceUpdated, MetadataDataSourceDeleted,
}

// EnvelopeSchema returns JSON Schema for the top-level delivery envelope.
func EnvelopeSchema() json.RawMessage {
	return mustRead("envelope.json")
}

// PayloadSchema returns JSON Schema for envelope.data of the given event type.
func PayloadSchema(eventType string) (json.RawMessage, bool) {
	if !ValidType(eventType) {
		return nil, false
	}
	raw := mustRead(schemaRelPath(eventType))
	return raw, true
}

// PayloadSchemas returns all per-type data payload schemas keyed by event type.
func PayloadSchemas() map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(payloadTypes))
	for _, typ := range payloadTypes {
		if raw, ok := PayloadSchema(typ); ok {
			out[typ] = raw
		}
	}
	return out
}

// PayloadSchemaTypes returns sorted event type names that have payload schemas.
func PayloadSchemaTypes() []string {
	out := append([]string(nil), payloadTypes...)
	sort.Strings(out)
	return out
}

// SchemaFileBytes returns raw schema JSON for external tooling.
// relPath is relative to schema/, e.g. envelope.json, records/after.insert.json.
func SchemaFileBytes(relPath string) ([]byte, error) {
	return schemaFS.ReadFile(schemaDir + "/" + relPath)
}

// schemaRelPath maps envelope.type to schema/<category>/<suffix>.json.
func schemaRelPath(eventType string) string {
	cat := CategoryOf(eventType)
	if cat == "" {
		return eventType + ".json"
	}
	suffix := strings.TrimPrefix(eventType, string(cat)+".")
	return string(cat) + "/" + suffix + ".json"
}

func mustRead(rel string) json.RawMessage {
	b, err := schemaFS.ReadFile(schemaDir + "/" + rel)
	if err != nil {
		panic("eventschema: missing " + rel + ": " + err.Error())
	}
	return json.RawMessage(b)
}
